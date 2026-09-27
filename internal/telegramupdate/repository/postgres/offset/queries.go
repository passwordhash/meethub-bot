package offset

import "github.com/jackc/pgx/v5"

func getQuery() string {
	return `-- getTelegramUpdateOffset
SELECT next_update_id FROM telegram_update_offset WHERE id = 1`
}

func setQuery(value int64) (string, pgx.NamedArgs) {
	return `-- setTelegramUpdateOffset
UPDATE telegram_update_offset SET next_update_id = GREATEST(next_update_id, @offset) WHERE id = 1 RETURNING next_update_id`, pgx.NamedArgs{"offset": value}
}
