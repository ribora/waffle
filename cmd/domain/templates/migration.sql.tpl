-- +goose Up
create table {{ .PluralsDomain }} (
    id bigint primary key,
    created_at timestamp(0) with time zone default now() not null,
    updated_at timestamp(0) with time zone default now() not null
);

create trigger {{ .PluralsDomain }}_updated_at
    before update
    on {{ .PluralsDomain }}
    for each row
execute procedure update_updated_at();

-- +goose Down
drop table {{ .PluralsDomain }};
