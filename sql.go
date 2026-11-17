// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

const InitSQL = `
-- schema for sqlite.

pragma foreign_keys = on;

begin transaction;

create table if not exists locations (
  id integer primary key autoincrement,
  country text not null,
  city text not null,
  state text
);

create table if not exists users (
  id text primary key not null, -- uuid
  email text,
  full_name text,
  updated_at timestamp not null -- utc, rfc3339
);

create unique index if not exists idx_users_email
on users(email collate nocase);

create table if not exists refresh_tokens (
  jti text primary key not null, -- uuid
  user_id text not null, -- uuid
  created_at timestamp not null, -- utc, rfc3339
  revoked integer not null default 0,
  foreign key (user_id) references users(id) on delete cascade,
  check (revoked in (0, 1))
);

create index if not exists idx_refresh_tokens_user_id
on refresh_tokens(user_id);

create table if not exists webauthn_credentials (
  id text primary key not null, -- base64url credential id
  user_id text not null, -- uuid
  credential text not null, -- json, marshaled webauthn.Credential
  last_used_at timestamp, -- utc, rfc3339
  created_at timestamp not null, -- utc, rfc3339
  foreign key (user_id) references users(id) on delete cascade
);

create index if not exists idx_webauthn_credentials_user_id
on webauthn_credentials(user_id);

create table if not exists webauthn_sessions (
  id text primary key not null, -- uuid, echoed back by the client between start and finish
  type text not null, -- ceremony type
  user_id text, -- uuid, nullable: login sessions have no user until finish
  email text, -- pending signup email, register sessions only
  full_name text, -- pending signup full name, register sessions only
  session_data text not null, -- json, marshaled webauthn.SessionData
  expires_at timestamp not null, -- utc, rfc3339
  created_at timestamp not null, -- utc, rfc3339
  foreign key (user_id) references users(id) on delete cascade,
  check (type in ('register', 'login'))
);

create index if not exists idx_webauthn_sessions_expires_at
on webauthn_sessions(expires_at);

create table if not exists candidates (
  id text primary key not null, -- uuid
  user_id text not null, -- uuid
  about text not null,
  pref_remote integer,
  pref_title_1 text,
  pref_title_2 text,
  pref_title_3 text,
  pref_location_1_id integer,
  pref_location_2_id integer,
  pref_location_3_id integer,
  last_recommended_at timestamp not null, -- utc, rfc3339
  unique(user_id),
  foreign key (user_id) references users(id) on delete cascade,
  foreign key (pref_location_1_id) references locations(id),
  foreign key (pref_location_2_id) references locations(id),
  foreign key (pref_location_3_id) references locations(id),
  check (pref_remote in (0, 1))
);

create table if not exists candidate_experiences (
  id text primary key not null, -- uuid
  candidate_id text not null, -- uuid
  title text not null,
  started_at timestamp not null, -- utc, rfc3339
  ended_at timestamp not null, -- utc, rfc3339
  description text,
  company text,
  experience_type text not null,
  skill_1 text,
  skill_2 text,
  skill_3 text,
  skill_4 text,
  skill_5 text,
  foreign key (candidate_id) references candidates(id) on delete cascade,
  check (experience_type in (
    'work',
    'education',
    'certification',
    'internship',
    'personal project',
    'entrepreneurship',
    'apprenticeship',
    'volunteering',
    'other'
  ))
);

create index if not exists idx_candidate_experiences_candidate
on candidate_experiences(candidate_id);

create table if not exists recruiters (
  id text primary key not null, -- uuid
  user_id text not null, -- uuid
  unique(user_id),
  foreign key (user_id) references users(id) on delete cascade
);

create table if not exists positions (
  id text primary key not null, -- uuid
  recruiter_id text not null, -- uuid
  title text not null,
  description text not null,
  company text,
  is_remote integer not null default 0,
  location_id integer,
  skill_1 text,
  skill_2 text,
  skill_3 text,
  skill_4 text,
  skill_5 text,
  is_active integer not null default 1,
  created_at timestamp not null, -- utc, rfc3339
  foreign key (recruiter_id) references recruiters(id) on delete cascade,
  foreign key (location_id) references locations(id),
  check (is_active in (0, 1)),
  check (is_remote in (0, 1))
);

create index if not exists idx_positions_active
on positions(is_active);

create table if not exists recommendations (
  id text primary key not null, -- uuid
  position_id text not null, -- uuid
  candidate_id text not null, -- uuid
  foreign key (position_id) references positions(id) on delete cascade,
  foreign key (candidate_id) references candidates(id) on delete cascade,
  unique(position_id, candidate_id)
);

create index if not exists idx_recommendations_candidate_position
on recommendations(candidate_id, position_id);

create index if not exists idx_recommendations_position
on recommendations(position_id);

create index if not exists idx_recommendations_candidate
on recommendations(candidate_id);

create index if not exists idx_recommendations_candidate_id
on recommendations(candidate_id, id);

create table if not exists candidate_reactions (
  recommendation_id text not null, -- uuid
  candidate_id text not null, -- uuid
  reaction_type text not null,
  reacted_at timestamp not null, -- utc, rfc3339
  primary key (recommendation_id, candidate_id),
  foreign key (recommendation_id) references recommendations(id) on delete cascade,
  foreign key (candidate_id) references candidates(id) on delete cascade,
  check (reaction_type in ('positive', 'negative'))
);

create index if not exists idx_candidate_reactions_recommendation
on candidate_reactions(recommendation_id);

create table if not exists recruiter_reactions (
  recommendation_id text not null, -- uuid
  recruiter_id text not null, -- uuid
  reaction_type text not null,
  reacted_at timestamp not null, -- utc, rfc3339
  primary key (recommendation_id, recruiter_id),
  foreign key (recommendation_id) references recommendations(id) on delete cascade,
  foreign key (recruiter_id) references recruiters(id) on delete cascade,
  check (reaction_type in ('positive', 'negative'))
);

create index if not exists idx_recruiter_reactions_recommendation
on recruiter_reactions(recommendation_id);

create table if not exists matches (
  candidate_id text not null, -- uuid
  position_id text not null, -- uuid
  matched_at timestamp not null, -- utc, rfc3339
  primary key (candidate_id, position_id),
  foreign key (candidate_id) references candidates(id) on delete cascade,
  foreign key (position_id) references positions(id) on delete cascade
);

-- positions fts setup

create virtual table if not exists positions_fts using fts5(
  id unindexed,
  title,
  description,
  company
);

create trigger if not exists positions_ai
after insert on positions
begin
  insert into positions_fts(id, title, description, company)
  values (new.id, new.title, new.description, new.company);
end;

create trigger if not exists positions_ad
after delete on positions
begin
  delete from positions_fts where id = old.id;
end;

create trigger if not exists positions_au
after update on positions
begin
  delete from positions_fts where id = old.id;
  insert into positions_fts(id, title, description, company)
  values (new.id, new.title, new.description, new.company);
end;

-- candidates fts setup

create virtual table if not exists candidates_fts using fts5(
  id unindexed,
  about
);

create trigger if not exists candidates_ai
after insert on candidates
begin
  insert into candidates_fts(id, about)
  values (new.id, new.about);
end;

create trigger if not exists candidates_ad
after delete on candidates
begin
  delete from candidates_fts where id = old.id;
end;

create trigger if not exists candidates_au
after update on candidates
begin
  delete from candidates_fts where id = old.id;
  insert into candidates_fts(id, about)
  values (new.id, new.about);
end;

commit;
`
