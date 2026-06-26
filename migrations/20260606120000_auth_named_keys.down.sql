-- Revert named auth entries back to bare strings (names are dropped):
--   api_key.keys:           [{"name": .., "key": "k1"}, ...]  -> ["k1", ...]
--   ip_validation.allowed_ips: [{"name": .., "ip": "1.2.3.4"}, ...] -> ["1.2.3.4", ...]

CREATE OR REPLACE FUNCTION auth_named_keys_down(auth jsonb) RETURNS jsonb AS $$
DECLARE
    methods     jsonb;
    method      jsonb;
    new_methods jsonb := '[]'::jsonb;
    sub         jsonb;
    elem        jsonb;
    new_arr     jsonb;
BEGIN
    IF auth IS NULL OR jsonb_typeof(auth) <> 'object' THEN
        RETURN auth;
    END IF;

    methods := auth->'methods';
    IF methods IS NULL OR jsonb_typeof(methods) <> 'array' THEN
        RETURN auth;
    END IF;

    FOR method IN SELECT * FROM jsonb_array_elements(methods) LOOP
        -- api_key.keys
        sub := method->'api_key';
        IF jsonb_typeof(sub) = 'object' AND jsonb_typeof(sub->'keys') = 'array' THEN
            new_arr := '[]'::jsonb;
            FOR elem IN SELECT * FROM jsonb_array_elements(sub->'keys') LOOP
                IF jsonb_typeof(elem) = 'object' THEN
                    new_arr := new_arr || to_jsonb(elem->>'key');
                ELSE
                    new_arr := new_arr || elem;
                END IF;
            END LOOP;
            method := jsonb_set(method, '{api_key,keys}', new_arr);
        END IF;

        -- ip_validation.allowed_ips
        sub := method->'ip_validation';
        IF jsonb_typeof(sub) = 'object' AND jsonb_typeof(sub->'allowed_ips') = 'array' THEN
            new_arr := '[]'::jsonb;
            FOR elem IN SELECT * FROM jsonb_array_elements(sub->'allowed_ips') LOOP
                IF jsonb_typeof(elem) = 'object' THEN
                    new_arr := new_arr || to_jsonb(elem->>'ip');
                ELSE
                    new_arr := new_arr || elem;
                END IF;
            END LOOP;
            method := jsonb_set(method, '{ip_validation,allowed_ips}', new_arr);
        END IF;

        new_methods := new_methods || method;
    END LOOP;

    RETURN jsonb_set(auth, '{methods}', new_methods);
END;
$$ LANGUAGE plpgsql;

UPDATE root     SET data = jsonb_set(data, '{auth}', auth_named_keys_down(data->'auth')) WHERE jsonb_typeof(data->'auth') = 'object';
UPDATE app      SET data = jsonb_set(data, '{auth}', auth_named_keys_down(data->'auth')) WHERE jsonb_typeof(data->'auth') = 'object';
UPDATE endpoint SET data = jsonb_set(data, '{auth}', auth_named_keys_down(data->'auth')) WHERE jsonb_typeof(data->'auth') = 'object';

DROP FUNCTION auth_named_keys_down(jsonb) CASCADE;

DELETE FROM snapshot;
