create index if not exists idx_embeddings_file_type_latest
  on embeddings (file_id, embedding_type, id desc)
  where file_id is not null;

create index if not exists idx_embeddings_frame_type_latest
  on embeddings (frame_id, embedding_type, id desc)
  where frame_id is not null;

create index if not exists idx_embeddings_image_visual_l2
  on embeddings using ivfflat (vector vector_l2_ops)
  with (lists = 100)
  where embedding_type = 'image_visual';

create index if not exists idx_embeddings_video_frame_visual_l2
  on embeddings using ivfflat (vector vector_l2_ops)
  with (lists = 100)
  where embedding_type = 'video_frame_visual';
