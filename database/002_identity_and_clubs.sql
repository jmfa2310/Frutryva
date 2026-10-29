-- Sprint 1: additive schema; existing users, clubs and sports remain intact.
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;
CREATE UNIQUE INDEX users_email_normalized ON users (lower(email));

CREATE TABLE auth_sessions (
 token_hash TEXT PRIMARY KEY,
 user_id UUID NOT NULL REFERENCES users(id),
 expires_at TIMESTAMPTZ NOT NULL,
 revoked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX auth_sessions_user ON auth_sessions(user_id);

CREATE TABLE auth_tokens (
 token_hash TEXT PRIMARY KEY,
 user_id UUID NOT NULL REFERENCES users(id),
 purpose TEXT NOT NULL CHECK(purpose IN ('verify_email','reset_password')),
 expires_at TIMESTAMPTZ NOT NULL,
 used_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX auth_tokens_user ON auth_tokens(user_id,purpose);

ALTER TABLE clubs ADD COLUMN archived_at TIMESTAMPTZ;
ALTER TABLE clubs ADD COLUMN published_branding_version INTEGER;

CREATE TABLE club_branding_drafts (
 club_id UUID PRIMARY KEY REFERENCES clubs(id),
 theme JSONB NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE club_branding_versions (
 club_id UUID NOT NULL REFERENCES clubs(id),
 version INTEGER NOT NULL CHECK(version > 0),
 theme JSONB NOT NULL,
 published_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(club_id,version)
);
ALTER TABLE clubs ADD CONSTRAINT clubs_published_branding_fk
 FOREIGN KEY(id,published_branding_version)
 REFERENCES club_branding_versions(club_id,version);

CREATE TABLE club_invitations (
 id UUID PRIMARY KEY,
 club_id UUID NOT NULL REFERENCES clubs(id),
 email TEXT NOT NULL,
 role TEXT NOT NULL CHECK(role IN ('director','coach','assistant')),
 token_hash TEXT UNIQUE NOT NULL,
 created_by UUID NOT NULL REFERENCES users(id),
 expires_at TIMESTAMPTZ NOT NULL,
 accepted_at TIMESTAMPTZ,
 revoked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX club_invitations_pending_email
 ON club_invitations(club_id,lower(email))
 WHERE accepted_at IS NULL AND revoked_at IS NULL;
CREATE INDEX club_invitations_club ON club_invitations(club_id);
