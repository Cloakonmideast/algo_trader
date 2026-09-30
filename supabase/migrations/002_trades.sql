-- Run once in the Supabase SQL editor, after 001_init.sql. Adds the Trades tab's table.
create table if not exists trades (
  id          bigint generated always as identity primary key,
  created_at  timestamptz   not null default now(),
  trade_date  date          not null,                  -- IST day of the trade
  symbol      text          not null check (symbol ~ '^[A-Z0-9&-]{1,30}$'),
  side        text          not null check (side in ('buy', 'sell')),
  price       numeric(12,2) check (price > 0),         -- optional on manual trades
  signal_id   bigint        references signals(id) on delete set null,
  signal      text,                                    -- signal name when recorded; null = manual
  unique (signal_id, side)                             -- one buy and one sell per signal
);
create index if not exists trades_date_idx   on trades (trade_date desc, id desc);
create index if not exists trades_signal_idx on trades (signal_id);

-- RLS on with no policies: only the service key (the website server) can read or write.
alter table trades enable row level security;
