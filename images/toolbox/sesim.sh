# The player's shell in the store's network. /etc/bash/bashrc sources it in
# interactive shells only, so `docker exec toolbox psql -c …` stays quiet.

cat <<'EOF'
Terminal na rede da loja do Mercado Lerdo.
  psql                  abre o banco, já autenticado (dentro dele: \q sai, \? ajuda)
  psql -c "SELECT 1"    roda uma consulta sem entrar no banco
EOF

# SQL typed at the bash prompt is the most common first mistake: answer with
# the way into the database instead of "command not found". A pasted query
# fails once per line, so the explanation is printed once per paste.
command_not_found_handle() {
	case "${1,,}" in
	select | with | explain | create | drop | alter | insert | update | delete | show | from | where | order | group | limit | join | begin | commit | rollback | vacuum | \\*)
		local now last
		now=$(date +%s)
		read -r last 2>/dev/null </tmp/.sesim-sql-hint
		echo "$now" >/tmp/.sesim-sql-hint 2>/dev/null
		if ((now - ${last:-0} > 2)); then
			printf '%s\n' "\"$1\" é SQL: ele roda dentro do banco, não no terminal." \
				"Abra o banco com psql e cole a consulta lá (ou rode: psql -c \"SELECT …\")." >&2
		fi
		;;
	*)
		printf 'bash: %s: comando não encontrado\n' "$1" >&2
		;;
	esac
	return 127
}
