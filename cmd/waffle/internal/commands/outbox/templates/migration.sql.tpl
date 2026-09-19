-- +goose Up
create table outbox (
    id             text primary key,
    subject        text not null,
    payload        bytea not null,
    correlation_id text not null default '',
    traceparent    text not null default '',
    tracestate     text not null default '',
    created_at     timestamp(6) with time zone not null default now(),
    claimed_at     timestamp(6) with time zone,
    claimed_by     text not null default '',
    claim_token    text,
    sent_at        timestamp(6) with time zone,
    attempts       int not null default 0,
    last_error     text not null default ''
);

create index outbox_claimable on outbox (id) where sent_at is null;
-- Partial on is-not-null and not on a sentinel: `claim_token = $1` implies it, so
-- the planner can use this index from a prepared statement, which a predicate like
-- `claim_token <> ''` would not let it do.
create index outbox_claim_token on outbox (claim_token) where claim_token is not null;
create index outbox_sent_at on outbox (sent_at) where sent_at is not null;

-- +goose Down
drop table outbox;
