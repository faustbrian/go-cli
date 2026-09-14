package cli

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maximumOutputRecords   = 1000
	maximumOutputBytes     = 1 << 20
	maximumOutputDepth     = 128
	maximumOutputElements  = 100_000
	maximumOutputTypeNodes = 4096
)

// OutputMode selects a stable presentation contract.
type OutputMode uint8

const (
	// OutputHuman emits plain text for people and pipes.
	OutputHuman OutputMode = iota
	// OutputJSON emits a versioned JSON envelope on stdout.
	OutputJSON
	// OutputQuiet suppresses successful informational output.
	OutputQuiet
)

// OutputPolicy controls one invocation's presentation contract.
type OutputPolicy struct {
	Mode    OutputMode
	NoColor bool
	Width   int
}

// Output buffers bounded invocation output until terminal success is known.
type Output struct {
	mu        sync.Mutex
	infos     []string
	dataJSON  json.RawMessage
	dataHuman string
	hasData   bool
	bytes     int
	dataBytes int
}

// Info records a bounded informational line.
func (output *Output) Info(message string) error {
	if output == nil {
		return newInternalError("write through a nil output", nil)
	}
	output.mu.Lock()
	defer output.mu.Unlock()
	renderedBytes := max(len(message), humanStringSize(message))
	if len(output.infos) >= maximumOutputRecords ||
		output.bytes+output.dataBytes+renderedBytes > maximumOutputBytes {
		return newClassifiedError(ErrorKindOutput, "output exceeds configured limit", nil, false)
	}
	output.infos = append(output.infos, message)
	output.bytes += renderedBytes

	return nil
}

// SetData records one success value for human or JSON rendering. Values are
// size-checked before serialization. Application-defined Marshaler,
// TextMarshaler, Stringer, Formatter, and error implementations are rejected
// because their work and allocation cannot be bounded by the runtime. Fields
// tagged with omitzero also reject application-defined IsZero callbacks.
func (output *Output) SetData(value any) error {
	if output == nil {
		return newInternalError("write through a nil output", nil)
	}
	size, err := validateOutputValue(value)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return newClassifiedError(ErrorKindOutput, "encode structured output", err, true)
	}
	human := fmt.Sprint(value)
	output.mu.Lock()
	defer output.mu.Unlock()
	dataBytes := max(len(encoded), len(human), size.json, size.human)
	if output.bytes+dataBytes > maximumOutputBytes {
		return newClassifiedError(ErrorKindOutput, "output exceeds configured limit", nil, false)
	}
	output.dataJSON = append(output.dataJSON[:0], encoded...)
	output.dataHuman = human
	output.hasData = true
	output.dataBytes = dataBytes

	return nil
}

var (
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	errorType         = reflect.TypeFor[error]()
	formatterType     = reflect.TypeFor[fmt.Formatter]()
	stringerType      = reflect.TypeFor[fmt.Stringer]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
	zeroCheckerType   = reflect.TypeFor[interface{ IsZero() bool }]()
	durationType      = reflect.TypeFor[time.Duration]()
	rawMessageType    = reflect.TypeFor[json.RawMessage]()
)

type outputSize struct {
	json  int
	human int
}

func validateOutputValue(value any) (outputSize, error) {
	size, err := estimateOutputSize(reflect.ValueOf(value), make(map[outputVisit]bool), 0)
	if err != nil {
		return outputSize{}, err
	}
	if outputSizeExceeded(size) {
		return outputSize{}, newClassifiedError(
			ErrorKindOutput, "output exceeds configured limit", nil, false,
		)
	}

	return size, nil
}

type outputVisit struct {
	typ reflect.Type
	ptr uintptr
}

func estimateOutputSize(
	value reflect.Value,
	path map[outputVisit]bool,
	depth int,
) (outputSize, error) {
	return estimateOutputSizeWithTypes(value, path, depth, newOutputTypeState())
}

func estimateOutputSizeWithTypes(
	value reflect.Value,
	path map[outputVisit]bool,
	depth int,
	types *outputTypeState,
) (outputSize, error) {
	if depth > maximumOutputDepth {
		return outputSize{}, newClassifiedError(
			ErrorKindOutput, "output exceeds configured depth", nil, false,
		)
	}
	if !value.IsValid() {
		return outputSize{json: 4, human: 5}, nil
	}
	typ := value.Type()
	if err := types.add(typ, 0); err != nil {
		return outputSize{}, err
	}
	if typ == durationType {
		return outputSize{json: 21, human: 32}, nil
	}
	if typ == rawMessageType {
		raw := value.Bytes()
		return outputSize{
			json:  rawMessageJSONSize(raw),
			human: boundedSum(boundedProduct(len(raw), 4), 2),
		}, nil
	}
	if hasCustomSerialization(typ) {
		return outputSize{}, newClassifiedError(
			ErrorKindOutput,
			"custom output serialization is not bounded",
			nil,
			false,
		)
	}

	switch value.Kind() { //nolint:exhaustive // Unsupported kinds use the bounded fallback.
	case reflect.Interface:
		if value.IsNil() {
			return outputSize{json: 4, human: 5}, nil
		}
		return estimateOutputSizeWithTypes(value.Elem(), path, outputChildDepth(depth), types)
	case reflect.Pointer:
		if value.IsNil() {
			return outputSize{json: 4, human: 5}, nil
		}
		visit := outputVisit{typ: typ, ptr: value.Pointer()}
		if path[visit] {
			return outputSize{}, newClassifiedError(
				ErrorKindOutput, "cyclic output cannot be bounded", nil, false,
			)
		}
		path[visit] = true
		size, err := estimateOutputSizeWithTypes(
			value.Elem(), path, outputChildDepth(depth), types,
		)
		delete(path, visit)
		size.human = max(
			boundedSum(size.human, 1),
			2+strconv.IntSize/4,
		)
		return size, err
	case reflect.String:
		return outputSize{
			json:  jsonStringSize(value.String()),
			human: humanStringSize(value.String()),
		}, nil
	case reflect.Bool:
		return outputSize{json: 5, human: 5}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		length := len(strconv.FormatInt(value.Int(), 10))
		return outputSize{json: length, human: length}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		length := len(strconv.FormatUint(value.Uint(), 10))
		return outputSize{json: length, human: length}, nil
	case reflect.Float32, reflect.Float64:
		return outputSize{json: 32, human: 32}, nil
	case reflect.Array, reflect.Slice:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return outputSize{json: 4, human: 2}, nil
		}
		if value.Len() > maximumOutputElements {
			return outputSize{}, newClassifiedError(
				ErrorKindOutput, "output exceeds configured collection limit", nil, false,
			)
		}
		visit := outputVisit{typ: typ}
		if value.Kind() == reflect.Slice {
			visit.ptr = value.Pointer()
			if path[visit] {
				return outputSize{}, newClassifiedError(
					ErrorKindOutput, "cyclic output cannot be bounded", nil, false,
				)
			}
			path[visit] = true
			defer delete(path, visit)
		}
		size := outputSize{json: 2, human: 2}
		for index := range value.Len() {
			item, err := estimateOutputSizeWithTypes(
				value.Index(index), path, outputChildDepth(depth), types,
			)
			if err != nil {
				return outputSize{}, err
			}
			size.json = boundedSum(size.json, item.json, outputSeparatorSize(index))
			size.human = boundedSum(size.human, item.human, outputSeparatorSize(index))
			if outputSizeExceeded(size) {
				return size, nil
			}
		}
		return size, nil
	case reflect.Map:
		if value.IsNil() {
			return outputSize{json: 4, human: 5}, nil
		}
		if value.Len() > maximumOutputElements {
			return outputSize{}, newClassifiedError(
				ErrorKindOutput, "output exceeds configured collection limit", nil, false,
			)
		}
		visit := outputVisit{typ: typ, ptr: value.Pointer()}
		if path[visit] {
			return outputSize{}, newClassifiedError(
				ErrorKindOutput, "cyclic output cannot be bounded", nil, false,
			)
		}
		path[visit] = true
		defer delete(path, visit)
		size := outputSize{json: 2, human: 5}
		iterator := value.MapRange()
		separator := 0
		for iterator.Next() {
			key, err := estimateMapKeySizeWithTypes(
				iterator.Key(), path, outputChildDepth(depth), types,
			)
			if err != nil {
				return outputSize{}, err
			}
			item, err := estimateOutputSizeWithTypes(
				iterator.Value(), path, outputChildDepth(depth), types,
			)
			if err != nil {
				return outputSize{}, err
			}
			size.json = boundedSum(size.json, key.json, item.json, 1, separator)
			size.human = boundedSum(size.human, key.human, item.human, 1, separator)
			separator = 1
			if outputSizeExceeded(size) {
				return size, nil
			}
		}
		return size, nil
	case reflect.Struct:
		size := outputSize{json: 2, human: 2}
		jsonSeparator := 0
		for index := range value.NumField() {
			field := typ.Field(index)
			item, err := estimateOutputSizeWithTypes(
				value.Field(index), path, outputChildDepth(depth), types,
			)
			if err != nil {
				return outputSize{}, err
			}
			size.human = boundedSum(size.human, item.human, outputSeparatorSize(index))
			if field.PkgPath != "" {
				continue
			}
			tag := field.Tag.Get("json")
			if tag == "-" {
				continue
			}
			name, options := parseJSONTag(tag)
			if !validJSONTagName(name) {
				name = ""
			}
			if jsonTagOption(options, "omitzero") && hasCustomZeroCheck(field.Type) {
				return outputSize{}, newClassifiedError(
					ErrorKindOutput,
					"custom output zero check is not bounded",
					nil,
					false,
				)
			}
			if name == "" {
				name = field.Name
			}
			itemJSON := item.json
			if jsonTagOption(options, "string") {
				itemJSON = boundedSum(boundedProduct(itemJSON, 6), 2)
			}
			size.json = boundedSum(
				size.json, jsonStringSize(name), itemJSON, 1, jsonSeparator,
			)
			jsonSeparator = 1
			if outputSizeExceeded(size) {
				return size, nil
			}
		}
		return size, nil
	default:
		return outputSize{json: 64, human: 64}, nil
	}
}

type outputTypeState struct {
	seen     map[reflect.Type]struct{}
	nodes    int
	metadata int
}

func newOutputTypeState() *outputTypeState {
	return &outputTypeState{seen: make(map[reflect.Type]struct{})}
}

func (state *outputTypeState) add(typ reflect.Type, depth int) error {
	if typ == nil {
		return nil
	}
	if _, exists := state.seen[typ]; exists {
		return nil
	}
	if depth > maximumOutputDepth {
		return newClassifiedError(
			ErrorKindOutput, "output encoder type graph exceeds configured depth", nil, false,
		)
	}
	state.nodes++
	if state.nodes > maximumOutputTypeNodes {
		return newClassifiedError(
			ErrorKindOutput, "output encoder type graph exceeds configured count", nil, false,
		)
	}
	state.seen[typ] = struct{}{}
	if typ.Implements(jsonMarshalerType) || typ.Implements(textMarshalerType) {
		return nil
	}

	nextDepth := outputChildDepth(depth)
	switch typ.Kind() { //nolint:exhaustive // Scalar and unsupported types have no child encoders.
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return state.add(typ.Elem(), nextDepth)
	case reflect.Map:
		return state.add(typ.Elem(), nextDepth)
	case reflect.Struct:
		for index := range typ.NumField() {
			field := typ.Field(index)
			state.nodes++
			state.metadata = boundedSum(
				state.metadata, len(field.Name), len(field.PkgPath), len(field.Tag),
			)
			if state.nodes > maximumOutputTypeNodes || state.metadata > maximumOutputBytes {
				return newClassifiedError(
					ErrorKindOutput, "output encoder type metadata exceeds configured limit", nil, false,
				)
			}
			if err := state.add(field.Type, nextDepth); err != nil {
				return err
			}
		}
	}

	return nil
}

func rawMessageJSONSize(raw []byte) int {
	if len(raw) > maximumOutputBytes {
		return maximumOutputBytes + 1
	}
	size := len(raw)
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '<', '>', '&':
			size = boundedSum(size, 5)
		case 0xe2:
			if index+2 < len(raw) && raw[index+1] == 0x80 &&
				(raw[index+2] == 0xa8 || raw[index+2] == 0xa9) {
				size = boundedSum(size, 3)
			}
		}
		if size > maximumOutputBytes {
			return size
		}
	}

	return size
}

func estimateMapKeySize(
	value reflect.Value,
	path map[outputVisit]bool,
	depth int,
) (outputSize, error) {
	return estimateMapKeySizeWithTypes(value, path, depth, newOutputTypeState())
}

func estimateMapKeySizeWithTypes(
	value reflect.Value,
	path map[outputVisit]bool,
	depth int,
	types *outputTypeState,
) (outputSize, error) {
	size, err := estimateOutputSizeWithTypes(value, path, depth, types)
	if err != nil {
		return outputSize{}, err
	}
	switch value.Kind() { //nolint:exhaustive // JSON map keys support only these scalar kinds.
	case reflect.String:
		size.json = jsonStringSize(value.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		size.json = len(strconv.FormatInt(value.Int(), 10)) + 2
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		size.json = len(strconv.FormatUint(value.Uint(), 10)) + 2
	default:
		return outputSize{}, newClassifiedError(
			ErrorKindOutput, "encode structured output: unsupported map key", nil, false,
		)
	}

	return size, nil
}

func hasCustomSerialization(typ reflect.Type) bool {
	if implementsCustomSerialization(typ) {
		return true
	}
	switch typ.Kind() { //nolint:exhaustive // Only pointers skip the pointer method-set check.
	case reflect.Pointer:
		return false
	default:
		return implementsCustomSerialization(reflect.PointerTo(typ))
	}
}

func implementsCustomSerialization(typ reflect.Type) bool {
	if typ.Implements(jsonMarshalerType) {
		return true
	}
	if typ.Implements(textMarshalerType) {
		return true
	}
	if typ.Implements(stringerType) {
		return true
	}
	if typ.Implements(formatterType) {
		return true
	}
	if typ.Implements(errorType) {
		return true
	}

	return false
}

func hasCustomZeroCheck(typ reflect.Type) bool {
	if typ.Implements(zeroCheckerType) {
		return true
	}
	if typ.Kind() == reflect.Pointer {
		return false
	}

	return reflect.PointerTo(typ).Implements(zeroCheckerType)
}

func parseJSONTag(tag string) (string, string) {
	name, options, found := strings.Cut(tag, ",")
	if !found {
		return tag, ""
	}

	return name, options
}

func jsonTagOption(options, target string) bool {
	for options != "" {
		option, remaining, found := strings.Cut(options, ",")
		if option == target {
			return true
		}
		if !found {
			return false
		}
		options = remaining
	}

	return false
}

func validJSONTagName(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range name {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", character):
		case !unicode.IsLetter(character) && !unicode.IsDigit(character):
			return false
		}
	}

	return true
}

func jsonStringSize(value string) int {
	size := 2
	for index := range value {
		character, width := utf8.DecodeRuneInString(value[index:])
		switch {
		case character == utf8.RuneError && width == 1:
			size = boundedSum(size, 6)
		case character == '\\' || character == '"':
			size = boundedSum(size, 2)
		case character < 0x20 || character == '<' || character == '>' || character == '&' ||
			character == '\u2028' || character == '\u2029':
			size = boundedSum(size, 6)
		default:
			size = boundedSum(size, width)
		}
		if size > maximumOutputBytes {
			return size
		}
	}

	return size
}

func humanStringSize(value string) int {
	if len(value) > maximumOutputBytes {
		return maximumOutputBytes + 1
	}
	size := 0
	for index := range value {
		character, width := utf8.DecodeRuneInString(value[index:])
		if character == utf8.RuneError && width == 1 {
			size = boundedSum(size, utf8.RuneLen(utf8.RuneError))
		} else if !isUnsafeTerminalRune(character) {
			size = boundedSum(size, width)
		}
		if size > maximumOutputBytes {
			return size
		}
	}

	return size
}

func boundedSum(values ...int) int {
	result := 0
	for _, value := range values {
		result += value
		if result > maximumOutputBytes {
			return maximumOutputBytes + 1
		}
	}

	return result
}

func boundedProduct(left, right int) int {
	if left != 0 && right > maximumOutputBytes/left {
		return maximumOutputBytes + 1
	}

	return left * right
}

func outputChildDepth(depth int) int {
	return depth + 1
}

func outputSeparatorSize(index int) int {
	switch index {
	case 0:
		return 0
	default:
		return 1
	}
}

func outputSizeExceeded(size outputSize) bool {
	if size.json > maximumOutputBytes {
		return true
	}
	if size.human > maximumOutputBytes {
		return true
	}

	return false
}

type outputSnapshot struct {
	infos     []string
	dataJSON  json.RawMessage
	dataHuman string
	hasData   bool
}

func (output *Output) snapshot() outputSnapshot {
	if output == nil {
		return outputSnapshot{}
	}
	output.mu.Lock()
	defer output.mu.Unlock()

	return outputSnapshot{
		infos:     cloneStrings(output.infos),
		dataJSON:  append(json.RawMessage(nil), output.dataJSON...),
		dataHuman: output.dataHuman,
		hasData:   output.hasData,
	}
}

func renderSuccess(writer io.Writer, policy OutputPolicy, output *Output) error {
	snapshot := output.snapshot()
	switch policy.Mode {
	case OutputQuiet:
		return nil
	case OutputJSON:
		envelope := struct {
			Schema string          `json:"schema"`
			OK     bool            `json:"ok"`
			Data   json.RawMessage `json:"data,omitempty"`
		}{Schema: "go-cli/v1", OK: true}
		if snapshot.hasData {
			envelope.Data = snapshot.dataJSON
		}
		return encodeAndWrite(writer, envelope)
	case OutputHuman:
		var builder strings.Builder
		for _, message := range snapshot.infos {
			builder.WriteString(sanitizeTerminal(message))
			builder.WriteByte('\n')
		}
		if snapshot.hasData {
			builder.WriteString(sanitizeTerminalMultiline(snapshot.dataHuman))
			builder.WriteByte('\n')
		}
		return writeAll(writer, []byte(builder.String()))
	default:
		return newInternalError("invalid output mode", nil)
	}
}

func renderFailure(stdout, stderr io.Writer, policy OutputPolicy, failure error) error {
	classified := classifiedError(failure)
	kind := ErrorKindCommand
	message := sanitizeTerminal(failure.Error())
	if classified != nil {
		kind = classified.Kind()
		message = sanitizeTerminal(classified.Error())
	}
	if policy.Mode == OutputJSON {
		envelope := struct {
			Schema string `json:"schema"`
			OK     bool   `json:"ok"`
			Error  struct {
				Kind    ErrorKind `json:"kind"`
				Message string    `json:"message"`
			} `json:"error"`
		}{Schema: "go-cli/v1", OK: false}
		envelope.Error.Kind = kind
		envelope.Error.Message = message

		return encodeAndWrite(stdout, envelope)
	}

	return writeAll(stderr, []byte("Error: "+message+"\n"))
}

func classifiedError(err error) *Error {
	if classified, ok := errors.AsType[*Error](err); ok {
		return classified
	}

	return nil
}

func encodeAndWrite(writer io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')

	return writeAll(writer, encoded)
}

func writeAll(writer io.Writer, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	written, err := writer.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}

	return nil
}

func sanitizeTerminal(value string) string {
	return strings.Map(func(character rune) rune {
		if isUnsafeTerminalRune(character) {
			return -1
		}

		return character
	}, value)
}

func sanitizeTerminalMultiline(value string) string {
	return strings.Map(func(character rune) rune {
		if character == '\n' || character == '\t' {
			return character
		}
		if isUnsafeTerminalRune(character) {
			return -1
		}

		return character
	}, value)
}

func isUnsafeTerminalRune(character rune) bool {
	return unicode.IsControl(character) || unicode.Is(unicode.Bidi_Control, character)
}
