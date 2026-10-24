-- +goose Up
-- +goose StatementBegin
do $create$
    begin
        create function update_updated_at()
            returns trigger as $trigger$
        begin
            new.updated_at = now();
            return new;
        end;
        $trigger$ language 'plpgsql';

exception
        when duplicate_function then
            null;
    end;
$create$;
-- +goose StatementEnd

-- +goose Down
