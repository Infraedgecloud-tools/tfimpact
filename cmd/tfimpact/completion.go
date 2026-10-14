package main

import (
	"fmt"
	"io"
)

// completion prints a completion script for one of the three shells people
// actually use. The scripts are hand-written: the command surface is five
// subcommands and a dozen flags, which does not need a framework.
func completion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "tfimpact completion: say which shell: bash, zsh or fish")
		return exitUsage
	}
	script, ok := completions[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "tfimpact completion: unknown shell %q (bash, zsh or fish)\n", args[0])
		return exitUsage
	}
	fmt.Fprint(stdout, script)
	return exitOK
}

var completions = map[string]string{
	"bash": `# tfimpact bash completion. Load with: source <(tfimpact completion bash)
_tfimpact() {
  local cur prev words cword
  _init_completion 2>/dev/null || { cur=${COMP_WORDS[COMP_CWORD]}; prev=${COMP_WORDS[COMP_CWORD-1]}; words=("${COMP_WORDS[@]}"); cword=$COMP_CWORD; }
  local commands="explain comment rules knowledge completion version help"
  local explain_flags="--format --output --json --sarif --fail-on --no-values --config --knowledge --quiet --color"
  if [ "$cword" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "$commands" -- "$cur") ); return
  fi
  case "${words[1]}" in
    explain)
      case "$prev" in
        --format) COMPREPLY=( $(compgen -W "markdown text json sarif" -- "$cur") ); return ;;
        --fail-on) COMPREPLY=( $(compgen -W "none low medium high critical" -- "$cur") ); return ;;
        --color) COMPREPLY=( $(compgen -W "auto always never" -- "$cur") ); return ;;
        --output|--json|--sarif|--config) COMPREPLY=( $(compgen -f -- "$cur") ); return ;;
        --knowledge) COMPREPLY=( $(compgen -d -- "$cur") ); return ;;
      esac
      case "$cur" in
        -*) COMPREPLY=( $(compgen -W "$explain_flags" -- "$cur") ) ;;
        *) COMPREPLY=( $(compgen -f -- "$cur") ) ;;
      esac ;;
    comment) COMPREPLY=( $(compgen -W "--body" -- "$cur") ) ;;
    completion) COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") ) ;;
  esac
}
complete -F _tfimpact tfimpact
`,
	"zsh": `#compdef tfimpact
# tfimpact zsh completion. Load with: source <(tfimpact completion zsh)
_tfimpact() {
  local -a commands
  commands=(
    'explain:explain a saved plan or its JSON'
    'comment:create or update the pull-request comment'
    'rules:list the rules'
    'knowledge:list or show AWS resource knowledge'
    'completion:print a completion script'
    'version:print the version'
  )
  if (( CURRENT == 2 )); then
    _describe 'command' commands; return
  fi
  case $words[2] in
    explain)
      _arguments \
        '--format[what to print]:format:(markdown text json sarif)' \
        '--output[write the Markdown report to a file]:file:_files' \
        '--json[write the findings JSON to a file]:file:_files' \
        '--sarif[write a SARIF log to a file]:file:_files' \
        '--fail-on[exit 1 at this severity]:level:(none low medium high critical)' \
        '--no-values[never print attribute values]' \
        '--config[suppressions file]:file:_files' \
        '--knowledge[knowledge directory]:dir:_files -/' \
        '--quiet[print nothing to stdout]' \
        '--color[colour in the text report]:mode:(auto always never)' \
        '*:plan:_files' ;;
    comment) _arguments '--body[comment body file]:file:_files' ;;
    completion) _arguments '1:shell:(bash zsh fish)' ;;
  esac
}
compdef _tfimpact tfimpact
`,
	"fish": `# tfimpact fish completion. Load with: tfimpact completion fish | source
complete -c tfimpact -f
complete -c tfimpact -n '__fish_use_subcommand' -a explain -d 'explain a saved plan or its JSON'
complete -c tfimpact -n '__fish_use_subcommand' -a comment -d 'create or update the pull-request comment'
complete -c tfimpact -n '__fish_use_subcommand' -a rules -d 'list the rules'
complete -c tfimpact -n '__fish_use_subcommand' -a knowledge -d 'list or show AWS resource knowledge'
complete -c tfimpact -n '__fish_use_subcommand' -a completion -d 'print a completion script'
complete -c tfimpact -n '__fish_use_subcommand' -a version -d 'print the version'
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -F
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l format -x -a 'markdown text json sarif'
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l output -r -F
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l json -r -F
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l sarif -r -F
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l fail-on -x -a 'none low medium high critical'
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l no-values
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l config -r -F
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l knowledge -r -a '(__fish_complete_directories)'
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l quiet
complete -c tfimpact -n '__fish_seen_subcommand_from explain' -l color -x -a 'auto always never'
complete -c tfimpact -n '__fish_seen_subcommand_from comment' -l body -r -F
complete -c tfimpact -n '__fish_seen_subcommand_from completion' -x -a 'bash zsh fish'
`,
}
