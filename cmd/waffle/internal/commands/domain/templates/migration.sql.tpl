-- +goose Up
create table {{ plural .Domain }} (
    id bigint primary key,
    created_at timestamp(0) with time zone default now() not null,
    updated_at timestamp(0) with time zone default now() not null
);

create trigger {{ plural .Domain }}_updated_at
    before update
    on {{ plural .Domain }}
    for each row
execute procedure update_updated_at();

-- +goose Down
drop table {{ plural .Domain }};
