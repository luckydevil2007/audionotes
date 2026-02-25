-- +goose Up
INSERT INTO notes (note_title, owner_id, note_path, lat, lon, excursion_id ) VALUES ('1st note', 1, 'storage/1.ogg',59.9371, 30.4263, 1);
INSERT INTO notes (note_title, owner_id, note_path, lat, lon ) VALUES ('2nd note', 1, 'storage/1.ogg',60.0, 30.0 );
INSERT INTO notes (note_title, owner_id, note_path, lat, lon ) VALUES ('3rd note', 1, 'storage/1.ogg',60.1, 30.1 );
INSERT INTO notes (note_title, owner_id, note_path, lat, lon ) VALUES ('4th note', 1, 'storage/1.ogg',60.2, 30.2 );
INSERT INTO notes (note_title, owner_id, note_path, lat, lon, excursion_id ) VALUES ('Metro', 1, 'storage/1.ogg',59.907, 30.483, 2 );
INSERT INTO notes (note_title, owner_id, note_path, lat, lon, excursion_id ) VALUES ('Me', 1, 'storage/1.ogg',59.904, 30.460, 2  );
INSERT INTO notes (note_title, owner_id, note_path, lat, lon, excursion_id ) VALUES ('5ka', 1, 'storage/1.ogg',59.903, 30.465, 2  );
INSERT INTO notes (note_title, owner_id, note_path, lat, lon, excursion_id ) VALUES ('Basik', 1, 'storage/1.ogg',59.920, 30.453, 2  );
INSERT INTO pathes (path_title, owner_id, path_points_id) VALUES ('1st Excursion', 1, ARRAY[1,2,3,4]);
INSERT INTO pathes (path_title, owner_id, path_points_id) VALUES ('Dybenko', 1, ARRAY[5,6,7,8]);
-- +goose Down

