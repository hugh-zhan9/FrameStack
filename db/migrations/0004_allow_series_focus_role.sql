alter table cluster_members
  drop constraint if exists chk_cluster_members_role;

alter table cluster_members
  add constraint chk_cluster_members_role
  check (role in ('cover', 'member', 'best_quality', 'duplicate_candidate', 'series_focus'));
