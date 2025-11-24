-- Remove foreign key constraint
ALTER TABLE user_recording_links
DROP CONSTRAINT IF EXISTS fk_user_recording_links_recording_id;


