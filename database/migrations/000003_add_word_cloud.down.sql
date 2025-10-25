-- Rollback word cloud functionality

DROP TABLE IF EXISTS word_frequencies CASCADE;
DROP TABLE IF EXISTS word_cloud_metadata CASCADE;
DROP FUNCTION IF EXISTS clean_word_for_cloud(TEXT);
