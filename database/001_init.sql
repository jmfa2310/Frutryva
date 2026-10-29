-- Fundação multi-clube e multidesporto. Apenas desenvolvimento inicial.
CREATE TABLE IF NOT EXISTS sports (
  id SMALLSERIAL PRIMARY KEY, code TEXT UNIQUE NOT NULL, name TEXT NOT NULL
);
INSERT INTO sports (code,name) VALUES
 ('football','Futebol'),('futsal','Futsal'),('volleyball','Voleibol'),
 ('basketball','Basquetebol'),('handball','Andebol')
ON CONFLICT (code) DO NOTHING;
CREATE TABLE IF NOT EXISTS sport_variants (
  id BIGSERIAL PRIMARY KEY, sport_id SMALLINT NOT NULL REFERENCES sports(id),
  name TEXT NOT NULL, players_on_field SMALLINT NOT NULL CHECK(players_on_field > 0),
  UNIQUE(sport_id,name)
);
INSERT INTO sport_variants (sport_id,name,players_on_field)
SELECT s.id,v.name,v.players FROM (VALUES
 ('football','Futebol 11',11),('football','Futebol 9',9),('football','Futebol 7',7),
 ('futsal','Futsal',5),('volleyball','Voleibol de pavilhão',6),
 ('basketball','Basquetebol 5x5',5),('handball','Andebol de pavilhão',7)
) AS v(code,name,players) JOIN sports s ON s.code=v.code
ON CONFLICT(sport_id,name) DO NOTHING;
CREATE TABLE IF NOT EXISTS users (
 id UUID PRIMARY KEY, email TEXT UNIQUE NOT NULL, display_name TEXT NOT NULL,
 password_hash TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS clubs (
 id UUID PRIMARY KEY, name TEXT NOT NULL, short_name TEXT,
 primary_color CHAR(7) NOT NULL DEFAULT '#00C8E8',
 secondary_color CHAR(7) NOT NULL DEFAULT '#07172B',
 accent_color CHAR(7) NOT NULL DEFAULT '#8BF000',
 logo_url TEXT, cover_url TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS club_memberships (
 club_id UUID NOT NULL REFERENCES clubs(id), user_id UUID NOT NULL REFERENCES users(id),
 role TEXT NOT NULL CHECK(role IN ('president','director','coach','assistant','staff')),
 PRIMARY KEY(club_id,user_id,role)
);
CREATE TABLE IF NOT EXISTS teams (
 id UUID PRIMARY KEY, club_id UUID NOT NULL REFERENCES clubs(id),
 sport_variant_id BIGINT NOT NULL REFERENCES sport_variants(id),
 name TEXT NOT NULL, season TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(id,club_id)
);
CREATE TABLE IF NOT EXISTS team_coaches (
 club_id UUID NOT NULL, team_id UUID NOT NULL, user_id UUID NOT NULL REFERENCES users(id),
 role TEXT NOT NULL DEFAULT 'coach', PRIMARY KEY(team_id,user_id),
 FOREIGN KEY(team_id,club_id) REFERENCES teams(id,club_id)
);
CREATE TABLE IF NOT EXISTS players (
 id UUID PRIMARY KEY, club_id UUID NOT NULL REFERENCES clubs(id),
 display_name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(id,club_id)
);
CREATE TABLE IF NOT EXISTS team_players (
 club_id UUID NOT NULL, team_id UUID NOT NULL, player_id UUID NOT NULL,
 shirt_number SMALLINT, PRIMARY KEY(team_id,player_id),
 FOREIGN KEY(team_id,club_id) REFERENCES teams(id,club_id),
 FOREIGN KEY(player_id,club_id) REFERENCES players(id,club_id)
);
CREATE TABLE IF NOT EXISTS exercises (
 id UUID PRIMARY KEY, owner_user_id UUID NOT NULL REFERENCES users(id),
 club_id UUID REFERENCES clubs(id), sport_variant_id BIGINT REFERENCES sport_variants(id),
 title TEXT NOT NULL, scene JSONB NOT NULL DEFAULT '{}'::jsonb,
 visibility TEXT NOT NULL DEFAULT 'private' CHECK(visibility IN ('private','club','public')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Fases seguintes: auth real, convites, jogos, estatísticas, loja, subscrições e auditoria.
