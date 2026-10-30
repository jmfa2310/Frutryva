-- Platform owners operate the SaaS and are never tenant club managers.
-- Retain all clubs, branding and historical membership records.
WITH assignments AS (
 SELECT u.id,u.assigned_club_id
 FROM users u JOIN platform_owners p ON p.user_id=u.id
 WHERE u.assigned_club_id IS NOT NULL
 FOR UPDATE OF u
), cleared AS (
 UPDATE users u SET assigned_club_id=NULL FROM assignments a
 WHERE u.id=a.id
 RETURNING u.id,a.assigned_club_id AS previous_club_id
)
INSERT INTO platform_events(actor_id,action,club_id,target_user_id)
SELECT id,'owner_club_assignment_removed',previous_club_id,id FROM cleared;
