alter table embeddings
  drop constraint if exists chk_embeddings_type;

alter table embeddings
  add constraint chk_embeddings_type
    check (embedding_type in ('image_visual', 'video_frame_visual', 'person_visual', 'face', 'search_text'));

create index if not exists idx_embeddings_person_visual_l2
  on embeddings using ivfflat (vector vector_l2_ops)
  with (lists = 100)
  where embedding_type = 'person_visual';
