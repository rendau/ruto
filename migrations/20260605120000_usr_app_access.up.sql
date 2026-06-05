ALTER TABLE usr
    ADD COLUMN all_apps boolean NOT NULL DEFAULT false,
    ADD COLUMN app_ids  text[]  NOT NULL DEFAULT '{}';

-- existing users keep full app access
UPDATE usr SET all_apps = true;
