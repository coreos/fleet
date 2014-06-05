_fleetctl_complete ()
{
  local cur

  COMPREPLY=()   # Array variable storing the possible completions.
  cur=${COMP_WORDS[COMP_CWORD]}
  prev=${COMP_WORDS[COMP_CWORD-1]}

  fleet_commands='cat debug-info destroy help journal list-machines
list-units load ssh start status stop submit unload verify version'

  if (( $COMP_CWORD <= 1 )); then
      __fleetctl_complete_subcommands
  fi
  # TODO: handle arguments for each command and options.
  return 0
}

__fleetctl_complete_subcommands ()
{
    case "$cur" in
        *)
            COMPREPLY=( $( compgen -W '$fleet_commands' -- $cur ) );;
    esac
}

complete -F _fleetctl_complete fleetctl
