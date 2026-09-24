-- Run once in the Supabase SQL editor.
create table if not exists signals (
  id          bigint generated always as identity primary key,
  fired_at    timestamptz   not null,
  trade_date  date          not null,          -- IST calendar date of fired_at
  token       bigint        not null,
  symbol      text          not null,
  strategy    text          not null,          -- family: 'RSI', 'IchimokuKinkoHyo'
  signal      text          not null,          -- exact variant
  direction   text          not null check (direction in ('long', 'short')),
  price       numeric(12,2) not null,
  rsi_daily   numeric(5,2),
  rsi_weekly  numeric(5,2),
  rsi_monthly numeric(5,2),
  created_at  timestamptz   not null default now(),
  unique (signal, token, trade_date)
);
create index if not exists signals_date_idx     on signals (trade_date desc, fired_at desc);
create index if not exists signals_strategy_idx on signals (strategy, trade_date);
create index if not exists signals_signal_idx   on signals (signal, trade_date);
create index if not exists signals_symbol_idx   on signals (symbol, trade_date);

create table if not exists kite_session (
  id           smallint    primary key default 1 check (id = 1),
  access_token text        not null,
  user_id      text        not null,
  created_at   timestamptz not null default now()
);

create table if not exists engine_status (
  id             smallint    primary key default 1 check (id = 1),
  last_seen      timestamptz not null,
  instruments    integer     not null,
  feed_connected boolean     not null,
  token_valid    boolean     not null
);

-- RLS on with no policies: the anon key can do nothing. Only the service key
-- (engine + website server) can read or write.
alter table signals       enable row level security;
alter table kite_session  enable row level security;
alter table engine_status enable row level security;
