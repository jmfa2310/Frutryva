-- Platform-managed, single-club accounts. Preserve all historical memberships.
ALTER TABLE users ADD COLUMN assigned_club_id UUID REFERENCES clubs(id);

-- Adopt only unambiguous active memberships. Accounts with several active clubs
-- remain unassigned until a platform owner explicitly chooses one.
WITH unambiguous AS (
 SELECT m.user_id,min(m.club_id::text)::uuid AS club_id
 FROM club_memberships m JOIN clubs c ON c.id=m.club_id
 WHERE c.archived_at IS NULL
 GROUP BY m.user_id HAVING count(DISTINCT m.club_id)=1
)
UPDATE users u SET assigned_club_id=a.club_id FROM unambiguous a WHERE u.id=a.user_id;
CREATE INDEX users_assigned_club ON users(assigned_club_id);

CREATE TABLE platform_owners (
 user_id UUID PRIMARY KEY REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE platform_events (
 id BIGSERIAL PRIMARY KEY,
 actor_id UUID NOT NULL REFERENCES users(id),
 action TEXT NOT NULL,
 club_id UUID REFERENCES clubs(id),
 target_user_id UUID REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
