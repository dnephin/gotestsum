#!/usr/bin/env bash
#
# do - run project tasks
#
set -eu -o pipefail

help_binary="Build the binary."
binary() {
    CGO_ENABLED=0 go build -o dist/gotestsum .
}

update-golden() {
    gotestsum ./testjson ./internal/junitxml -test.update-golden
}

help_clean="Removes artifacts"
clean() {
    rm -rf dist/
}

help_test="Run the test suite"
test() {
    gotestsum -- ${GOTESTFLAGS:-} ./...
}

help_lint="Run golanci-lint to lint go files."
lint() {
    golangci-lint run --verbose ${GOLINTFLAGS:-}
}

help_godoc="Run godoc to read documentation."
godoc() {
    local url=http://localhost:6060/pkg/github.com/circleci/build-agent/
    command -v xdg-open && xdg-open $url &
    command -v open && open $url &
    command godoc -http=:6060
}

### START FRAMEWORK ###
# Do Version 0.0.3
help_self_update="Update the framework from a file.

Usage: $0 self-update FILENAME
"
self-update() {
    local source="$1"
    local selfpath="${BASH_SOURCE[0]}"
    cp "$selfpath" "$selfpath.bak"
    local pattern='/### START FRAMEWORK/,/END FRAMEWORK ###$/'
    (sed "${pattern}d" "$selfpath"; sed -n "${pattern}p" "$source") \
        > "$selfpath.new"
    mv "$selfpath.new" "$selfpath"
    chmod --reference="$selfpath.bak" "$selfpath"
}

help_completion="Print shell completion function for this script."
completion() {
    case "$(basename $SHELL)" in
      bash)
        (echo
        echo '_dotslashdo_completions() { '
        echo '  COMPREPLY=($(compgen -W "$('$0' list)" "${COMP_WORDS[1]}"))'
        echo '}'
        echo 'complete -F _dotslashdo_completions '$0
        );;
    esac
}

list() {
    declare -F | awk '{print $3}'
}

help_help="Print help text, or detailed help for a task."
help() {
    local item="${1-}"
    if [ -n "${item}" ]; then
      local help_name="help_${item//-/_}"
      echo "${!help_name-}"
      return
    fi

    type -t help-text-intro > /dev/null && help-text-intro
    for item in $(list); do
      local help_name="help_${item//-/_}"
      local text="${!help_name-}"
      [ -n "$text" ] && printf "%-20s\t%s\n" $item "$(echo "$text" | head -1)"
    done
}

case "${1-}" in
  list) list;;
  ""|"help") help "${2-}";;
  *) "$@";;
esac
### END FRAMEWORK ###
