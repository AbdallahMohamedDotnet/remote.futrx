#!/usr/bin/env bash

health_check_now_ms() {
    date +%s%3N
}

health_check_request() {
    local url="$1" max_time="$2"
    curl -fsS --max-time "$max_time" "$url" >/dev/null 2>&1
}

health_check_sleep() {
    sleep "$1"
}

health_check_seconds_for_ms() {
    local milliseconds="$1"
    printf '%d.%03d' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
}

# Polls an HTTP endpoint without allowing an individual request or retry sleep
# to extend past the overall wall-clock deadline.
wait_for_http_health() {
    local url="$1" timeout_seconds="$2"
    local now_ms deadline_ms remaining_ms request_ms sleep_ms
    local request_seconds sleep_seconds

    case "$timeout_seconds" in
        ''|0|*[!0-9]*) return 2 ;;
    esac

    now_ms="$(health_check_now_ms)"
    deadline_ms="$((now_ms + timeout_seconds * 1000))"
    while true; do
        now_ms="$(health_check_now_ms)"
        remaining_ms="$((deadline_ms - now_ms))"
        [ "$remaining_ms" -gt 0 ] || return 1

        request_ms=3000
        [ "$remaining_ms" -ge "$request_ms" ] || request_ms="$remaining_ms"
        request_seconds="$(health_check_seconds_for_ms "$request_ms")"
        if health_check_request "$url" "$request_seconds"; then
            return 0
        fi

        now_ms="$(health_check_now_ms)"
        remaining_ms="$((deadline_ms - now_ms))"
        [ "$remaining_ms" -gt 0 ] || return 1
        sleep_ms=1000
        [ "$remaining_ms" -ge "$sleep_ms" ] || sleep_ms="$remaining_ms"
        sleep_seconds="$(health_check_seconds_for_ms "$sleep_ms")"
        health_check_sleep "$sleep_seconds"
    done
}
