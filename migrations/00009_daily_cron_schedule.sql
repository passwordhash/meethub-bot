-- +goose Up
ALTER TABLE channels ADD COLUMN IF NOT EXISTS schedule_cron TEXT;
UPDATE channels
SET schedule_cron = schedule_minute::TEXT || ' ' || schedule_hour::TEXT || ' * * ' || (
    SELECT string_agg(CASE WHEN weekday = 7 THEN '0' ELSE weekday::TEXT END, ',')
    FROM unnest(schedule_weekdays) AS days(weekday)
)
WHERE schedule_hour IS NOT NULL AND cardinality(schedule_weekdays) > 0;
