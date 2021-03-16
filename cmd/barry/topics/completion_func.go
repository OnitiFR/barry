package topics

const (
	bashCompletionFunc = `
__barry_get_config() {
    __barry_config=$(eval $COMP_LINE --get-config-filename)
}

__internal_list_projects() {
    local barry_output out
	__barry_get_config
	if barry_output=$(barry --config $__barry_config project list --basic 2>/dev/null); then
        out=($(echo "${barry_output}"))
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__internal_list_files() {
    local barry_output out project_name
	project_name="$1"
	__barry_get_config
	if barry_output=$(barry --config $__barry_config project list "$project_name" --basic 2>/dev/null); then
        out=($(echo "${barry_output}"))
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__internal_file_download() {
	local prev_prev=${COMP_WORDS[COMP_CWORD-2]}
    if [ "$prev" =  "download" ]; then
		__internal_list_projects
    elif [ "$prev_prev" =  "download" ]; then
        __internal_list_files $prev
    fi
}

__custom_func() {
    case ${last_command} in
		barry_project_list)
            __internal_list_projects
            return
            ;;
		barry_file_download)
			__internal_file_download
            return
            ;;
        *)
            ;;
    esac
}
`
)
