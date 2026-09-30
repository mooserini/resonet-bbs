-- Dragon Tavern Legends now has a turn-based forest, and a full day of
-- fights doesn't fit in the old 3-minute door session. Boards still on the
-- shipped 180s default move to 30 minutes; a sysop-chosen limit is left alone.
UPDATE door_configs
SET max_run_seconds = 1800
WHERE door_id = 'dragon-tavern-legends' AND max_run_seconds = 180;
