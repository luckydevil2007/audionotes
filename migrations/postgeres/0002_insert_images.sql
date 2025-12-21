-- +goose Up
INSERT INTO notes (note_title, owner_id, note_path, lat, lon ) VALUES ('1st note', 1, 'storage/1.ogg',59.9371, 30.4263 );
INSERT INTO notes (note_title, owner_id, note_path, lat, lon ) VALUES ('1st note', 1, 'storage/1.ogg',60.0, 30.0 );
INSERT INTO pathes (path_title, owner_id, path_points_id) VALUES ('1st Excursion', 1, ARRAY[1,2]);
-- +goose Down

