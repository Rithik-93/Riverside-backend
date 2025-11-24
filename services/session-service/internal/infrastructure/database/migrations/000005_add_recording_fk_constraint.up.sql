-- Clean up orphaned user_recording_links that don't have corresponding recordings
DELETE FROM user_recording_links 
WHERE recording_id NOT IN (SELECT recording_id FROM recordings);

-- Add foreign key constraint to enforce referential integrity
ALTER TABLE user_recording_links
ADD CONSTRAINT fk_user_recording_links_recording_id 
FOREIGN KEY (recording_id) REFERENCES recordings(recording_id) 
ON DELETE CASCADE;


