-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS channel_daily_times (
    channel_id BIGINT NOT NULL REFERENCES channels(channel_id),
    weekday SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    starts_at TIME NOT NULL,
    PRIMARY KEY (channel_id, weekday)
);
ALTER TABLE channels ADD COLUMN IF NOT EXISTS post_offset_minutes INT NOT NULL DEFAULT 60 CHECK (post_offset_minutes BETWEEN 0 AND 1439);
ALTER TABLE channels ADD COLUMN IF NOT EXISTS schedule_reset_pending BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE publication_slots ADD COLUMN IF NOT EXISTS publish_alerted_at TIMESTAMPTZ;

DO $$
DECLARE
    channel RECORD;
    fields TEXT[];
    token TEXT;
    bounds TEXT[];
    days INT[];
    day INT;
    cron_minutes INT;
    daily_minutes INT;
    convertible BOOLEAN;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'channels' AND column_name = 'schedule_cron'
    ) THEN
        RETURN;
    END IF;
    FOR channel IN SELECT channel_id, schedule_cron FROM channels WHERE btrim(coalesce(schedule_cron, '')) <> '' LOOP
        fields := regexp_match(btrim(channel.schedule_cron), '^(\d{1,2})\s+(\d{1,2})\s+\*\s+\*\s+(\S+)$');
        convertible := fields IS NOT NULL AND fields[1]::INT < 60 AND fields[2]::INT < 24;
        days := '{}';
        IF convertible THEN
            FOREACH token IN ARRAY string_to_array(fields[3], ',') LOOP
                IF token = '*' THEN
                    days := days || ARRAY[0, 1, 2, 3, 4, 5, 6];
                    CONTINUE;
                END IF;
                bounds := regexp_match(token, '^([0-7])(?:-([0-7]))?$');
                IF bounds IS NULL OR bounds[1]::INT > coalesce(bounds[2]::INT, bounds[1]::INT) THEN
                    convertible := FALSE;
                    EXIT;
                END IF;
                FOR day IN bounds[1]::INT..coalesce(bounds[2]::INT, bounds[1]::INT) LOOP
                    days := days || (day % 7);
                END LOOP;
            END LOOP;
        END IF;
        IF NOT convertible THEN
            UPDATE channels SET schedule_reset_pending = TRUE WHERE channel_id = channel.channel_id;
            CONTINUE;
        END IF;
        cron_minutes := fields[2]::INT * 60 + fields[1]::INT;
        daily_minutes := greatest(cron_minutes, 600);
        INSERT INTO channel_daily_times (channel_id, weekday, starts_at)
        SELECT channel.channel_id, weekday, make_time(daily_minutes / 60, daily_minutes % 60, 0)
        FROM unnest(days) AS cron_days(weekday)
        ON CONFLICT (channel_id, weekday) DO NOTHING;
        UPDATE channels SET post_offset_minutes = daily_minutes - cron_minutes WHERE channel_id = channel.channel_id;
    END LOOP;
END
$$;

ALTER TABLE channels DROP COLUMN IF EXISTS schedule_cron;
ALTER TABLE channels DROP COLUMN IF EXISTS schedule_weekdays;
ALTER TABLE channels DROP COLUMN IF EXISTS schedule_hour;
ALTER TABLE channels DROP COLUMN IF EXISTS schedule_minute;
ALTER TABLE channels DROP COLUMN IF EXISTS next_override;
-- +goose StatementEnd
