#!/usr/bin/env bash
set -euo pipefail

readonly benchmark_pattern='^BenchmarkEquivalent(Cold|Repeated)Invocation$|^BenchmarkParsingFloorFlag$'
readonly conformance_pattern='^(TestComparisonOutputMatchesOwnedBoundsAndWriterContract|TestEquivalentBenchmarkRunnersIsolateRepeatedState|TestEquivalentBenchmarkRunnersShareObservableContract|TestEquivalentBenchmarkRunnersShareStructuralFailure|TestOwnedParserMatchesTheFormerCobraAdapter)$'
readonly benchmark_duration='100ms'
readonly sample_count='10'
readonly process_count='10'
readonly test_timeout='110s'
readonly process_timeout_seconds='120'
readonly benchstat_version='v0.0.0-20260709024250-82a0b07e230d'
readonly expected_benchmarks=(
  BenchmarkEquivalentColdInvocation/go-cli
  BenchmarkEquivalentColdInvocation/cobra
  BenchmarkEquivalentColdInvocation/urfave-cli-v3
  BenchmarkEquivalentColdInvocation/kong
  BenchmarkEquivalentRepeatedInvocation/go-cli
  BenchmarkEquivalentRepeatedInvocation/cobra
  BenchmarkEquivalentRepeatedInvocation/urfave-cli-v3
  BenchmarkEquivalentRepeatedInvocation/kong
  BenchmarkParsingFloorFlag
)

run_with_process_timeout() {
  /usr/bin/perl -e '
    use strict;
    use warnings;
    my $seconds = shift @ARGV;
    my $child = fork();
    die "fork failed: $!\n" unless defined $child;
    if ($child == 0) {
      setpgrp(0, 0) or die "setpgrp failed: $!\n";
      exec @ARGV;
      die "exec failed: $!\n";
    }
    my $termination = 0;
    my $terminate = sub {
      kill "TERM", -$child;
      select undef, undef, undef, 0.5;
      kill "KILL", -$child;
    };
    local $SIG{ALRM} = sub { $termination = 124; $terminate->() };
    local $SIG{INT} = sub { $termination = 130; $terminate->() };
    local $SIG{TERM} = sub { $termination = 143; $terminate->() };
    alarm $seconds;
    my $waited;
    do { $waited = waitpid($child, 0) } while $waited == -1 && $!{EINTR};
    my $status = $?;
    alarm 0;
    if ($termination == 124) {
      print STDERR "process exceeded ${seconds}s timeout\n";
    }
    if (kill 0, -$child) {
      $terminate->();
    }
    exit $termination if $termination;
    exit(($status & 127) ? 128 + ($status & 127) : $status >> 8);
  ' "$@"
}

if [[ $# -ne 1 ]]; then
  printf 'usage: %s OUTPUT_DIRECTORY\n' "${0##*/}" >&2
  exit 2
fi

cache_directories=()
for variable in GOCACHE GOMODCACHE GOTMPDIR; do
  if [[ -z "${!variable:-}" ]]; then
    printf '%s must name a task-owned disposable directory\n' "${variable}" >&2
    exit 2
  fi
  cache_directory="${!variable}"
  case "${cache_directory}" in
    /*) ;;
    *) printf '%s must be an absolute task-owned disposable directory\n' "${variable}" >&2; exit 2 ;;
  esac
  case "${cache_directory}" in
    /|/tmp|/private/tmp)
      printf '%s is too broad to clean safely: %s\n' "${variable}" "${cache_directory}" >&2
      exit 2
      ;;
  esac
  if [[ -e "${cache_directory}" && ! -d "${cache_directory}" ]]; then
    printf '%s must be absent or a directory: %s\n' "${variable}" "${cache_directory}" >&2
    exit 2
  fi
  if [[ -d "${cache_directory}" ]] &&
    find "${cache_directory}" -mindepth 1 -maxdepth 1 -print -quit | grep -q .; then
    printf '%s must be empty before capture: %s\n' "${variable}" "${cache_directory}" >&2
    exit 2
  fi
  cache_directories+=("${cache_directory}")
done

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
final_output_directory="$1"
if [[ "${final_output_directory}" != /* ]]; then
  printf 'OUTPUT_DIRECTORY must be an absolute path outside the repository\n' >&2
  exit 2
fi
output_parent="$(cd "$(dirname "${final_output_directory}")" && pwd -P)"
final_output_directory="${output_parent}/$(basename "${final_output_directory}")"
case "${final_output_directory}/" in
  "${repository_root}/"*)
    printf 'OUTPUT_DIRECTORY must be outside the repository\n' >&2
    exit 2
    ;;
esac
output_directory=''
caches_cleaned=false
cleanup_caches() {
  local directory
  local cleanup_status=0
  if [[ "${caches_cleaned}" == true ]]; then
    return 0
  fi
  for directory in "${cache_directories[@]}"; do
    [[ -e "${directory}" ]] || continue
    chmod -R u+w "${directory}" 2>/dev/null || cleanup_status=1
    find "${directory}" -depth -delete 2>/dev/null || cleanup_status=1
  done
  if [[ "${cleanup_status}" -eq 0 ]]; then
    caches_cleaned=true
  fi
  return "${cleanup_status}"
}
cleanup_output() {
  [[ -n "${output_directory}" && -e "${output_directory}" ]] || return 0
  chmod -R u+w "${output_directory}" 2>/dev/null || true
  find "${output_directory}" -depth -delete
}
finish() {
  local status=$?
  trap - EXIT INT TERM
  if ! cleanup_caches; then
    printf 'failed to remove one or more task-owned cache directories\n' >&2
    status=1
  fi
  if ! cleanup_output; then
    printf 'failed to remove partial benchmark evidence\n' >&2
    status=1
  fi
  exit "${status}"
}
interrupted() {
  trap - EXIT INT TERM
  cleanup_caches || printf 'failed to remove one or more task-owned cache directories\n' >&2
  cleanup_output || printf 'failed to remove partial benchmark evidence\n' >&2
  exit 130
}
trap finish EXIT
trap interrupted INT TERM
mkdir -p "${GOCACHE}" "${GOMODCACHE}" "${GOTMPDIR}"
if [[ -e "${final_output_directory}" ]]; then
  printf 'OUTPUT_DIRECTORY must not already exist: %s\n' "${final_output_directory}" >&2
  exit 2
fi
output_directory="$(mktemp -d "${output_parent}/.$(basename "${final_output_directory}").staging.XXXXXX")"
cd "${repository_root}"

captured_go_env() {
  run_with_process_timeout "${process_timeout_seconds}" env GOWORK=off go env "$1"
}

observed_go_version="$(captured_go_env GOVERSION)"
if [[ "${observed_go_version}" != 'go1.27.0' ]]; then
  printf 'benchmark evidence requires go1.27.0; found %s\n' "${observed_go_version}" >&2
  exit 1
fi

stable_identity() {
  local source_fingerprint
  source_fingerprint="$(./scripts/benchmark-input-fingerprint.sh)"
  printf 'source_base_head=%s\n' "$(git rev-parse HEAD)"
  printf 'source_fingerprint=%s\n' "${source_fingerprint}"
  printf 'source_state_begin\n'
  git status --short --untracked-files=all
  printf 'source_state_end\n'
  printf 'go_version=%s\n' "$(captured_go_env GOVERSION)"
  printf 'goos=%s\n' "$(captured_go_env GOOS)"
  printf 'goarch=%s\n' "$(captured_go_env GOARCH)"
  printf 'go386=%s\n' "$(captured_go_env GO386)"
  printf 'goamd64=%s\n' "$(captured_go_env GOAMD64)"
  printf 'goarm=%s\n' "$(captured_go_env GOARM)"
  printf 'goarm64=%s\n' "$(captured_go_env GOARM64)"
  printf 'gomips=%s\n' "$(captured_go_env GOMIPS)"
  printf 'gomips64=%s\n' "$(captured_go_env GOMIPS64)"
  printf 'goppc64=%s\n' "$(captured_go_env GOPPC64)"
  printf 'goriscv64=%s\n' "$(captured_go_env GORISCV64)"
  printf 'gowasm=%s\n' "$(captured_go_env GOWASM)"
  printf 'cgo_enabled=%s\n' "$(captured_go_env CGO_ENABLED)"
  printf 'goexperiment=%s\n' "$(captured_go_env GOEXPERIMENT)"
  printf 'goflags=%s\n' "$(captured_go_env GOFLAGS)"
  printf 'gomaxprocs=%s\n' "${GOMAXPROCS:-}"
  printf 'gogc=%s\n' "${GOGC:-}"
  printf 'gomemlimit=%s\n' "${GOMEMLIMIT:-}"
  printf 'godebug=%s\n' "${GODEBUG:-}"
  printf 'operating_system=%s\n' "$(uname -srv)"
  printf 'machine=%s\n' "$(uname -m)"
  local cpu=''
  local power_source='unknown'
  if [[ "$(uname -s)" == 'Darwin' ]]; then
    cpu="$(sysctl -n machdep.cpu.brand_string 2>/dev/null || true)"
    power_source="$(pmset -g batt 2>/dev/null | sed -n '1p' || true)"
  elif command -v lscpu >/dev/null 2>&1; then
    cpu="$(LC_ALL=C lscpu | awk -F: '/^(Model name|Architecture|Vendor ID):/ {gsub(/^[[:space:]]+/, "", $2); values = values (values ? "; " : "") $1 "=" $2} END {print values}')"
  elif [[ -r /proc/cpuinfo ]]; then
    cpu="$(awk -F ': ' '/^(model name|Processor|Hardware|cpu model|machine)/ {print $2; exit}' /proc/cpuinfo)"
  elif command -v sysctl >/dev/null 2>&1; then
    cpu="$(sysctl -n hw.model 2>/dev/null || true)"
  fi
  printf 'cpu=%s\n' "${cpu:-unknown ($(uname -a))}"
  printf 'power_source=%s\n' "${power_source:-unknown}"
  printf 'power_mode_begin\n'
  if [[ "$(uname -s)" == 'Darwin' ]] && command -v pmset >/dev/null 2>&1; then
    pmset -g custom 2>/dev/null || printf 'unavailable\n'
  elif [[ -r /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor ]]; then
    printf 'cpu_governor=%s\n' "$(< /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor)"
  else
    printf 'unavailable\n'
  fi
  printf 'power_mode_end\n'
  printf 'conformance_pattern=%s\n' "${conformance_pattern}"
  printf 'benchmark_pattern=%s\n' "${benchmark_pattern}"
  printf 'benchmark_duration=%s\n' "${benchmark_duration}"
  printf 'sample_count=%s\n' "${sample_count}"
  printf 'process_count=%s\n' "${process_count}"
  printf 'samples_per_process=1\n'
  printf 'test_timeout=%s\n' "${test_timeout}"
  printf 'process_timeout_seconds=%s\n' "${process_timeout_seconds}"
  printf 'benchstat_version=%s\n' "${benchstat_version}"
  printf 'competitor_revisions_begin\n'
  cat benchmarks/competitors.txt
  printf 'competitor_revisions_end\n'
  printf 'modules_begin\n'
  run_with_process_timeout "${process_timeout_seconds}" env \
    GOWORK="${repository_root}/go.work" go list -m all
  printf 'modules_end\n'
}

before_identity="${output_directory}/identity-before.txt"
after_identity="${output_directory}/identity-after.txt"
raw_results="${output_directory}/raw.txt"
statistical_summary="${output_directory}/benchstat.txt"
conformance_results="${output_directory}/conformance.txt"

stable_identity >"${before_identity}"
./scripts/benchmark-input-fingerprint.sh --manifest >"${output_directory}/source-manifest.txt"
printf 'started_utc=%s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" >"${output_directory}/capture.txt"
printf 'conformance_command=run_with_process_timeout %s env GOWORK=%s/go.work go test ./benchmarks/... -run %s -count=1 -timeout=%s\n' \
  "${process_timeout_seconds}" "\$PWD" "${conformance_pattern}" "${test_timeout}" >>"${output_directory}/capture.txt"
printf 'command=for process in 1..%s; run_with_process_timeout %s env GOWORK=%s/go.work go test ./benchmarks/... -run ^$ -bench %s -benchmem -benchtime=%s -count=1 -timeout=%s\n' \
  "${process_count}" "${process_timeout_seconds}" "${repository_root}" "${benchmark_pattern}" "${benchmark_duration}" "${test_timeout}" \
  >>"${output_directory}/capture.txt"
printf 'analysis_command=split raw.txt by comparison class; run_with_process_timeout %s env GOWORK=off go run golang.org/x/perf/cmd/benchstat@%s for each class\n' \
  "${process_timeout_seconds}" "${benchstat_version}" >>"${output_directory}/capture.txt"

set +e
run_with_process_timeout "${process_timeout_seconds}" env \
  GOWORK="${repository_root}/go.work" go test ./benchmarks/... \
  -run "${conformance_pattern}" -count=1 -timeout="${test_timeout}" 2>&1 | tee "${conformance_results}"
conformance_pipeline_status=("${PIPESTATUS[@]}")
if [[ ${conformance_pipeline_status[0]} -eq 0 && ${conformance_pipeline_status[1]} -eq 0 ]]; then
  : >"${raw_results}"
  benchmark_pipeline_status=(0 0)
  for ((process = 1; process <= process_count; process++)); do
    run_with_process_timeout "${process_timeout_seconds}" env \
      GOWORK="${repository_root}/go.work" go test ./benchmarks/... -run '^$' \
      -bench "${benchmark_pattern}" -benchmem \
      -benchtime="${benchmark_duration}" -count=1 -timeout="${test_timeout}" \
      2>&1 | tee -a "${raw_results}"
    benchmark_pipeline_status=("${PIPESTATUS[@]}")
    if [[ ${benchmark_pipeline_status[0]} -ne 0 || ${benchmark_pipeline_status[1]} -ne 0 ]]; then
      break
    fi
  done
else
  benchmark_pipeline_status=(0 0)
fi
set -e

stable_identity >"${after_identity}"
printf 'finished_utc=%s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
  >>"${output_directory}/capture.txt"
before_identity_digest="$(shasum -a 256 "${before_identity}" | awk '{print $1}')"
after_identity_digest="$(shasum -a 256 "${after_identity}" | awk '{print $1}')"
printf 'identity_before_sha256=%s\n' "${before_identity_digest}" \
  >>"${output_directory}/capture.txt"
printf 'identity_after_sha256=%s\n' "${after_identity_digest}" \
  >>"${output_directory}/capture.txt"
if cmp -s "${before_identity}" "${after_identity}"; then
  printf 'identity_match=true\n' >>"${output_directory}/capture.txt"
else
  printf 'identity_match=false\n' >>"${output_directory}/capture.txt"
  printf 'benchmark identity changed during capture\n' >&2
  diff -u "${before_identity}" "${after_identity}" >&2 || true
  exit 1
fi
if [[ ${conformance_pipeline_status[0]} -ne 0 || ${conformance_pipeline_status[1]} -ne 0 ]]; then
  printf 'conformance capture failed (test=%d, tee=%d)\n' \
    "${conformance_pipeline_status[0]}" "${conformance_pipeline_status[1]}" >&2
  exit 1
fi
if [[ ${benchmark_pipeline_status[0]} -ne 0 || ${benchmark_pipeline_status[1]} -ne 0 ]]; then
  printf 'benchmark capture failed (test=%d, tee=%d)\n' \
    "${benchmark_pipeline_status[0]}" "${benchmark_pipeline_status[1]}" >&2
  exit 1
fi

benchmark_rows=0
for expected in "${expected_benchmarks[@]}"; do
  observed="$(awk -v expected="${expected}" '
    $1 ~ ("^" expected "-[0-9]+$") { count++ }
    END { print count + 0 }
  ' "${raw_results}")"
  if [[ "${observed}" -ne "${sample_count}" ]]; then
    printf 'benchmark sample count for %s = %s, want %s\n' \
      "${expected}" "${observed}" "${sample_count}" >&2
    exit 1
  fi
  benchmark_rows=$((benchmark_rows + observed))
done
all_benchmark_rows="$(awk '$1 ~ /^Benchmark/ { count++ } END { print count + 0 }' "${raw_results}")"
if [[ "${all_benchmark_rows}" -ne "${benchmark_rows}" ]]; then
  printf 'unexpected benchmark rows = %s, want %s\n' \
    "${all_benchmark_rows}" "${benchmark_rows}" >&2
  exit 1
fi

comparison_raw="${output_directory}/comparison-raw.txt"
baseline_raw="${output_directory}/baseline-raw.txt"
awk '$1 !~ /^BenchmarkParsingFloorFlag-/ { print }' "${raw_results}" >"${comparison_raw}"
awk '$1 !~ /^BenchmarkEquivalent/ { print }' "${raw_results}" >"${baseline_raw}"
{
  printf 'comparison_class=common-denominator comparison\n'
  (
    cd "${output_directory}"
    run_with_process_timeout "${process_timeout_seconds}" env GOWORK=off \
      go run "golang.org/x/perf/cmd/benchstat@${benchstat_version}" comparison-raw.txt
  )
  printf '\ncomparison_class=raw baseline\n'
  (
    cd "${output_directory}"
    run_with_process_timeout "${process_timeout_seconds}" env GOWORK=off \
      go run "golang.org/x/perf/cmd/benchstat@${benchstat_version}" baseline-raw.txt
  )
} | tee "${statistical_summary}"
find "${comparison_raw}" "${baseline_raw}" -delete

mv "${before_identity}" "${output_directory}/identity.txt"
find "${after_identity}" -delete
if ! cleanup_caches; then
  printf 'failed to remove one or more task-owned cache directories\n' >&2
  exit 1
fi
if [[ -e "${final_output_directory}" ]]; then
  printf 'OUTPUT_DIRECTORY appeared during capture: %s\n' "${final_output_directory}" >&2
  exit 1
fi
mv "${output_directory}" "${final_output_directory}"
output_directory=''
