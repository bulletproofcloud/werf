#!/bin/bash

set -e

arg_werf_bin_path="${1:?ERROR: werf bin path should be specified as the first argument.}"

source_path="$(realpath "${BASH_SOURCE[0]}")"
project_dir="$(dirname $source_path)/../.."
docs_dir="$project_dir/docs"

function regen() {
  # regen CLI partials, pages and sidebar
  HOME='~' DOCKER_DEFAULT_PLATFORM='' $arg_werf_bin_path docs --dir="$project_dir" --log-terminal-width=100
}

function create_documentation_sidebar() {
  sidebar_documentation_path="$docs_dir/_data/sidebars/documentation.yml"
  sidebar_cli_partial_path="$docs_dir/_data/sidebars/_cli.yml"
  sidebar_documentation_partial_path="$docs_dir/_data/sidebars/_documentation.yml"

  cat << EOF > "$sidebar_documentation_path"
# This file is generated by "regen.sh" command.
# DO NOT EDIT!

# This is your sidebar TOC. The sidebar code loops through sections here and provides the appropriate formatting.

EOF
  {
    cat "$sidebar_cli_partial_path"
    echo
    sed 's/"\#\!\*cli"/*cli/' "$sidebar_documentation_partial_path"
  } >> "$sidebar_documentation_path"
}

regen
create_documentation_sidebar
