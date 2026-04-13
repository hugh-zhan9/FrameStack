package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"path/filepath"

	"idea/internal/app"
	"idea/internal/clusterreview"
	"idea/internal/clusters"
	"idea/internal/config"
	"idea/internal/database"
	"idea/internal/embeddings"
	"idea/internal/filehash"
	"idea/internal/files"
	"idea/internal/filetags"
	"idea/internal/httpserver"
	"idea/internal/jobexecutor"
	"idea/internal/jobrunner"
	"idea/internal/mediaextract"
	"idea/internal/quality"
	"idea/internal/reveal"
	"idea/internal/review"
	"idea/internal/samecontent"
	"idea/internal/sameperson"
	"idea/internal/sameseries"
	"idea/internal/scanner"
	"idea/internal/searchdoc"
	"idea/internal/systemstatus"
	"idea/internal/tags"
	"idea/internal/tasks"
	"idea/internal/trash"
	"idea/internal/understand"
	"idea/internal/volumes"
	"idea/internal/workerclient"
)

func main() {
	cfg := config.Load()
	application := app.New(cfg, nil)
	workerHealthChecker := workerclient.HealthChecker{
		Client: workerclient.Client{
			Command: cfg.WorkerCommand,
			Script:  cfg.WorkerScript,
		},
	}
	statusService := systemstatus.Service{
		EnableDatabase: cfg.EnableDatabase,
		RequireWorker:  cfg.RunJobWorker,
		Worker:         workerHealthChecker,
	}

	if cfg.EnableDatabase {
		db, err := database.Open(cfg.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		migrator := database.NewRunner(db)
		eventRecorder := tasks.PostgresJobEventRecorder{Execer: database.SQLExecer{DB: db}}
		volumeStore := volumes.NewPostgresStoreFromDB(db, db)
		tagStore := tags.NewPostgresStoreFromDB(db)
		clusterStore := clusters.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db})
		fileStore := files.NewPostgresStoreFromDB(db)
		revealStore := reveal.NewPostgresStoreFromDB(db)
		trashStore := trash.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db})
		reviewStore := review.PostgresStore{Execer: database.SQLExecer{DB: db}}
		samePersonService := sameperson.Service{
			Store: sameperson.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
		}
		application = app.New(cfg, migrator)
		application.SetTaskSummaryProvider(tasks.NewPostgresSummaryProviderFromDB(db))
		application.SetJobListProvider(tasks.NewPostgresJobListProviderFromDB(db))
		application.SetJobCreator(tasks.EventRecordingJobCreator{
			Creator:  tasks.NewPostgresJobCreatorFromDB(db),
			Recorder: eventRecorder,
		})
		application.SetJobRetrier(tasks.EventRecordingJobRetrier{
			Retrier:  tasks.NewPostgresJobRetrierFromDB(db),
			Recorder: eventRecorder,
		})
		application.SetJobEventListProvider(tasks.NewPostgresJobEventListProviderFromDB(db))
		application.SetVolumeListProvider(volumeListProvider{store: volumeStore})
		application.SetVolumeCreator(volumeCreator{store: volumeStore})
		application.SetVolumeScanner(volumeScanner{
			creator: tasks.EventRecordingJobCreator{
				Creator:  tasks.NewPostgresJobCreatorFromDB(db),
				Recorder: eventRecorder,
			},
		})
		application.SetTagListProvider(tagListProvider{store: tagStore})
		application.SetClusterSummaryProvider(clusterSummaryProvider{store: clusterStore})
		application.SetClusterListProvider(clusterListProvider{store: clusterStore})
		application.SetClusterDetailProvider(clusterDetailProvider{store: clusterStore})
		application.SetClusterReviewer(clusterReviewer{
			service: clusterreview.Service{
				Clusters:    clusterStore,
				Actions:     review.Service{Store: reviewStore},
				StatusStore: clusterStore,
			},
		})
		application.SetFileListProvider(fileListProvider{store: fileStore})
		application.SetFileDetailProvider(fileDetailProvider{store: fileStore})
		application.SetFileContentProvider(fileContentProvider{store: fileStore})
		application.SetFileTrasher(trash.Service{
			Store: trashStore,
			Mover: trash.MacOSMover{},
		})
		application.SetFileRevealer(reveal.Service{
			Store:    revealStore,
			Revealer: reveal.MacOSRevealer{},
		})
		application.SetFileReviewer(fileReviewer{
			service: review.Service{Store: reviewStore},
		})
		application.SetFileTagCreator(fileTagCreator{
			service: filetags.Service{
				Store:      filetags.PostgresStore{Execer: database.SQLExecer{DB: db}},
				SamePerson: samePersonService,
			},
		})
		application.SetFileJobRunner(fileJobRunner{
			understanding: understandingJobEnqueuer{
				ensurer: tasks.NewPostgresJobEnsurerFromDB(db),
			},
			embeddings: embeddingJobEnqueuer{
				ensurer: tasks.NewPostgresJobEnsurerFromDB(db),
			},
			sameContent: sameContentJobEnqueuer{
				ensurer: tasks.NewPostgresJobEnsurerFromDB(db),
			},
			sameSeries: sameSeriesJobEnqueuer{
				ensurer: tasks.NewPostgresJobEnsurerFromDB(db),
			},
			samePerson: samePersonJobEnqueuer{
				ensurer: tasks.NewPostgresJobEnsurerFromDB(db),
			},
		})
		statusService.Database = database.HealthChecker{DB: db}

		if cfg.RunJobWorker {
			jobEnsurer := tasks.NewPostgresJobEnsurerFromDB(db)
			searchDocService := searchdoc.Service{
				Store: searchdoc.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
			}
			sameContentService := samecontent.Service{
				Store: samecontent.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
			}
			sameSeriesService := sameseries.Service{
				Store: sameseries.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
			}
			fileHashService := filehash.Service{
				Store: filehash.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
				SameContentEnqueuer: sameContentJobEnqueuer{
					ensurer: jobEnsurer,
				},
			}
			qualityService := quality.Service{
				Store: quality.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
			}
			embeddingService := embeddings.Service{
				Store: embeddings.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
				Embedder: embeddings.WorkerEmbedder{
					Client: workerclient.Client{
						Command: cfg.WorkerCommand,
						Script:  cfg.WorkerScript,
					},
				},
			}
			understandService := understand.Service{
				Store: understand.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
				Analyzer: understand.WorkerAnalyzer{
					Client: workerclient.Client{
						Command: cfg.WorkerCommand,
						Script:  cfg.WorkerScript,
					},
				},
				PersonEmbeddingEnqueuer: embeddingJobEnqueuer{
					ensurer: jobEnsurer,
				},
				SamePersonEnqueuer: samePersonJobEnqueuer{
					ensurer: jobEnsurer,
				},
			}
			mediaExtractService := mediaextract.Service{
				Store:      mediaextract.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
				VideoProbe: mediaextract.FFprobeVideoProbe{},
				FrameExtractor: mediaextract.FFmpegVideoFrameExtractor{
					OutputRoot: filepath.Join("tmp", "previews"),
				},
				ThumbnailRoot: filepath.Join("tmp", "thumbnails"),
				SearchDocEnqueuer: searchDocJobEnqueuer{
					ensurer: jobEnsurer,
				},
				UnderstandingEnqueuer: understandingJobEnqueuer{
					ensurer: jobEnsurer,
				},
				QualityEnqueuer: qualityJobEnqueuer{
					ensurer: jobEnsurer,
				},
				EmbeddingEnqueuer: embeddingJobEnqueuer{
					ensurer: jobEnsurer,
				},
				SameContentEnqueuer: sameContentJobEnqueuer{
					ensurer: jobEnsurer,
				},
				SameSeriesEnqueuer: sameSeriesJobEnqueuer{
					ensurer: jobEnsurer,
				},
			}
			scanService := scanner.Service{
				Store: scanner.NewPostgresStoreFromDB(db, database.SQLExecer{DB: db}),
				Enqueuer: scannerJobEnqueuer{
					ensurer: jobEnsurer,
				},
			}
			runner := jobrunner.Runner{
				WorkerName: cfg.JobWorkerName,
				Store:      tasks.NewPostgresQueueStoreFromDB(db),
				Executor: jobexecutor.Executor{
					Scanner:          scanService,
					Extractor:        mediaExtractService,
					SearchIndexer:    searchDocService,
					Understander:     understandService,
					QualityEvaluator: qualityService,
					Embeddings:       embeddingService,
					FileHasher:       fileHashService,
					SameContent:      sameContentService,
					SameSeries:       sameSeriesService,
					SamePerson:       samePersonService,
					Fallback:         tasks.NoopExecutor{},
				},
				Recorder: eventRecorder,
			}
			go func() {
				if err := runner.Run(context.Background(), cfg.JobPollInterval); err != nil {
					log.Printf("job runner stopped: %v", err)
				}
			}()
		}
	}

	application.SetWorkerChecker(workerHealthChecker)
	application.SetSystemStatusProvider(statusService)

	if err := application.Bootstrap(context.Background()); err != nil {
		log.Fatal(err)
	}

	log.Printf("server listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, application.Handler()); err != nil {
		log.Fatal(err)
	}
}

type volumeListProvider struct {
	store volumes.PostgresStore
}

func (p volumeListProvider) ListVolumes(ctx context.Context) ([]httpserver.VolumeDTO, error) {
	items, err := p.store.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]httpserver.VolumeDTO, 0, len(items))
	for _, item := range items {
		result = append(result, httpserver.VolumeDTO{
			ID:          item.ID,
			DisplayName: item.DisplayName,
			MountPath:   item.MountPath,
			IsOnline:    item.IsOnline,
		})
	}
	return result, nil
}

type volumeCreator struct {
	store volumes.PostgresStore
}

func (p volumeCreator) CreateVolume(ctx context.Context, input httpserver.CreateVolumeRequest) (httpserver.VolumeDTO, error) {
	item, err := p.store.CreateVolume(ctx, volumes.CreateVolumeInput{
		DisplayName: input.DisplayName,
		MountPath:   input.MountPath,
	})
	if err != nil {
		return httpserver.VolumeDTO{}, err
	}
	return httpserver.VolumeDTO{
		ID:          item.ID,
		DisplayName: item.DisplayName,
		MountPath:   item.MountPath,
		IsOnline:    item.IsOnline,
	}, nil
}

type volumeScanner struct {
	creator tasks.EventRecordingJobCreator
}

func (p volumeScanner) EnqueueVolumeScan(ctx context.Context, volumeID int64) (tasks.Job, error) {
	payload, err := json.Marshal(map[string]int64{
		"volume_id": volumeID,
	})
	if err != nil {
		return tasks.Job{}, err
	}
	return p.creator.CreateJob(ctx, tasks.CreateJobInput{
		JobType:    "scan_volume",
		Priority:   50,
		TargetType: "volume",
		TargetID:   volumeID,
		Payload:    payload,
	})
}

type tagListProvider struct {
	store tags.PostgresStore
}

func (p tagListProvider) ListTags(ctx context.Context, input httpserver.TagListRequest) ([]httpserver.TagDTO, error) {
	items, err := p.store.ListTags(ctx, tags.ListOptions{
		Namespace: input.Namespace,
		Limit:     input.Limit,
	})
	if err != nil {
		return nil, err
	}
	result := make([]httpserver.TagDTO, 0, len(items))
	for _, item := range items {
		result = append(result, httpserver.TagDTO{
			Namespace:   item.Namespace,
			Name:        item.Name,
			DisplayName: item.DisplayName,
			FileCount:   item.FileCount,
		})
	}
	return result, nil
}

type clusterListProvider struct {
	store clusters.PostgresStore
}

type clusterSummaryProvider struct {
	store clusters.PostgresStore
}

func (p clusterSummaryProvider) SummarizeClusters(ctx context.Context) ([]httpserver.ClusterSummaryDTO, error) {
	items, err := p.store.SummarizeClusters(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]httpserver.ClusterSummaryDTO, 0, len(items))
	for _, item := range items {
		result = append(result, httpserver.ClusterSummaryDTO{
			ClusterType:  item.ClusterType,
			Status:       item.Status,
			ClusterCount: item.ClusterCount,
			MemberCount:  item.MemberCount,
		})
	}
	return result, nil
}

func (p clusterListProvider) ListClusters(ctx context.Context, input httpserver.ClusterListRequest) ([]httpserver.ClusterDTO, error) {
	items, err := p.store.ListClusters(ctx, clusters.ListOptions{
		ClusterType: input.ClusterType,
		Status:      input.Status,
		Limit:       input.Limit,
	})
	if err != nil {
		return nil, err
	}
	result := make([]httpserver.ClusterDTO, 0, len(items))
	for _, item := range items {
		result = append(result, httpserver.ClusterDTO{
			ID:          item.ID,
			ClusterType: item.ClusterType,
			Title:       item.Title,
			Confidence:  item.Confidence,
			Status:      item.Status,
			CoverFileID: item.CoverFileID,
			MemberCount: item.MemberCount,
			StrongMemberCount: item.StrongMemberCount,
			TopMemberScore: item.TopMemberScore,
			PersonVisualCount: item.PersonVisualCount,
			GenericVisualCount: item.GenericVisualCount,
			TopEvidenceType: item.TopEvidenceType,
			CreatedAt:   item.CreatedAt,
		})
	}
	return result, nil
}

type clusterDetailProvider struct {
	store clusters.PostgresStore
}

func (p clusterDetailProvider) GetClusterDetail(ctx context.Context, clusterID int64) (httpserver.ClusterDetailDTO, error) {
	item, err := p.store.GetClusterDetail(ctx, clusterID)
	if err != nil {
		return httpserver.ClusterDetailDTO{}, err
	}
	result := httpserver.ClusterDetailDTO{
		ClusterDTO: httpserver.ClusterDTO{
			ID:          item.ID,
			ClusterType: item.ClusterType,
			Title:       item.Title,
			Confidence:  item.Confidence,
			Status:      item.Status,
			CoverFileID: item.CoverFileID,
			MemberCount: item.MemberCount,
			StrongMemberCount: item.StrongMemberCount,
			TopMemberScore: item.TopMemberScore,
			CreatedAt:   item.CreatedAt,
		},
		PersonVisualCount:  item.PersonVisualCount,
		GenericVisualCount: item.GenericVisualCount,
		TopEvidenceType:    item.TopEvidenceType,
	}
	for _, member := range item.Members {
		result.Members = append(result.Members, httpserver.ClusterMemberDTO{
			FileID:               member.FileID,
			FileName:             member.FileName,
			AbsPath:              member.AbsPath,
			MediaType:            member.MediaType,
			Role:                 member.Role,
			Score:                member.Score,
			QualityTier:          member.QualityTier,
			HasFace:              member.HasFace,
			SubjectCount:         member.SubjectCount,
			CaptureType:          member.CaptureType,
			EmbeddingType:        member.EmbeddingType,
			EmbeddingProvider:    member.EmbeddingProvider,
			EmbeddingModel:       member.EmbeddingModel,
			EmbeddingVectorCount: member.EmbeddingVectorCount,
		})
	}
	return result, nil
}

type clusterReviewer struct {
	service clusterreview.Service
}

func (p clusterReviewer) ApplyClusterReviewAction(ctx context.Context, clusterID int64, input httpserver.FileReviewActionRequest) error {
	return p.service.ApplyClusterAction(ctx, clusterID, review.FileActionInput{
		ActionType: input.ActionType,
		Note:       input.Note,
	})
}

func (p clusterReviewer) UpdateClusterStatus(ctx context.Context, clusterID int64, status string) error {
	return p.service.UpdateClusterStatus(ctx, clusterID, status)
}

type fileListProvider struct {
	store files.PostgresStore
}

func (p fileListProvider) ListFiles(ctx context.Context, input httpserver.FileListRequest) ([]httpserver.FileDTO, error) {
	items, err := p.store.ListFiles(ctx, files.ListOptions{
		Limit:         input.Limit,
		Offset:        input.Offset,
		Query:         input.Query,
		MediaType:     input.MediaType,
		QualityTier:   input.QualityTier,
		ReviewAction:  input.ReviewAction,
		Status:        input.Status,
		VolumeID:      input.VolumeID,
		TagNamespace:  input.TagNamespace,
		Tag:           input.Tag,
		ClusterType:   input.ClusterType,
		ClusterStatus: input.ClusterStatus,
		Sort:          input.Sort,
	})
	if err != nil {
		return nil, err
	}
	result := make([]httpserver.FileDTO, 0, len(items))
	for _, item := range items {
		result = append(result, httpserver.FileDTO{
			ID:           item.ID,
			VolumeID:     item.VolumeID,
			AbsPath:      item.AbsPath,
			FileName:     item.FileName,
			MediaType:    item.MediaType,
			Status:       item.Status,
			SizeBytes:    item.SizeBytes,
			UpdatedAt:    item.UpdatedAt,
			Width:        item.Width,
			Height:       item.Height,
			DurationMS:   item.DurationMS,
			Format:       item.Format,
			Container:    item.Container,
			FPS:          item.FPS,
			Bitrate:      item.Bitrate,
			VideoCodec:   item.VideoCodec,
			AudioCodec:   item.AudioCodec,
			QualityScore: item.QualityScore,
			QualityTier:  item.QualityTier,
			ReviewAction: item.ReviewAction,
			TagNames:     item.TagNames,
			HasPreview:   item.HasPreview,
		})
	}
	return result, nil
}

type fileDetailProvider struct {
	store files.PostgresStore
}

func (p fileDetailProvider) GetFileDetail(ctx context.Context, fileID int64) (httpserver.FileDetailDTO, error) {
	item, err := p.store.GetFileDetail(ctx, fileID)
	if err != nil {
		return httpserver.FileDetailDTO{}, err
	}
	result := httpserver.FileDetailDTO{
		FileDTO: httpserver.FileDTO{
			ID:           item.ID,
			VolumeID:     item.VolumeID,
			AbsPath:      item.AbsPath,
			FileName:     item.FileName,
			MediaType:    item.MediaType,
			Status:       item.Status,
			SizeBytes:    item.SizeBytes,
			UpdatedAt:    item.UpdatedAt,
			Width:        item.Width,
			Height:       item.Height,
			DurationMS:   item.DurationMS,
			Format:       item.Format,
			Container:    item.Container,
			FPS:          item.FPS,
			Bitrate:      item.Bitrate,
			VideoCodec:   item.VideoCodec,
			AudioCodec:   item.AudioCodec,
			QualityScore: item.QualityScore,
			QualityTier:  item.QualityTier,
			ReviewAction: item.ReviewAction,
		},
	}
	for _, history := range item.PathHistory {
		result.PathHistory = append(result.PathHistory, httpserver.PathHistoryDTO{
			AbsPath:   history.AbsPath,
			EventType: history.EventType,
			SeenAt:    history.SeenAt,
		})
	}
	for _, analysis := range item.CurrentAnalyses {
		result.CurrentAnalyses = append(result.CurrentAnalyses, httpserver.CurrentAnalysisDTO{
			AnalysisType: analysis.AnalysisType,
			Status:       analysis.Status,
			Summary:      analysis.Summary,
			QualityScore: analysis.QualityScore,
			QualityTier:  analysis.QualityTier,
			CreatedAt:    analysis.CreatedAt,
		})
	}
	for _, tag := range item.Tags {
		result.Tags = append(result.Tags, httpserver.FileTagDTO{
			Namespace:   tag.Namespace,
			Name:        tag.Name,
			DisplayName: tag.DisplayName,
			Source:      tag.Source,
			Confidence:  tag.Confidence,
		})
	}
	for _, action := range item.ReviewActions {
		result.ReviewActions = append(result.ReviewActions, httpserver.ReviewActionDTO{
			ActionType: action.ActionType,
			Note:       action.Note,
			CreatedAt:  action.CreatedAt,
		})
	}
	for _, cluster := range item.Clusters {
		result.Clusters = append(result.Clusters, httpserver.FileClusterDTO{
			ID:          cluster.ClusterID,
			ClusterType: cluster.ClusterType,
			Title:       cluster.Title,
			Status:      cluster.Status,
		})
	}
	for _, embedding := range item.Embeddings {
		result.Embeddings = append(result.Embeddings, httpserver.EmbeddingInfoDTO{
			EmbeddingType: embedding.EmbeddingType,
			Provider:      embedding.Provider,
			ModelName:     embedding.ModelName,
			VectorCount:   embedding.VectorCount,
		})
	}
	for _, frame := range item.VideoFrames {
		result.VideoFrames = append(result.VideoFrames, httpserver.VideoFrameDTO{
			TimestampMS: frame.TimestampMS,
			FrameRole:   frame.FrameRole,
			PHash:       frame.PHash,
		})
	}
	return result, nil
}

type fileReviewer struct {
	service review.Service
}

func (p fileReviewer) ApplyFileReviewAction(ctx context.Context, fileID int64, input httpserver.FileReviewActionRequest) error {
	return p.service.ApplyFileAction(ctx, fileID, review.FileActionInput{
		ActionType: input.ActionType,
		Note:       input.Note,
	})
}

type fileContentProvider struct {
	store files.PostgresStore
}

func (p fileContentProvider) GetFileContent(ctx context.Context, fileID int64) (httpserver.FileContent, error) {
	item, err := p.store.GetFileContent(ctx, fileID)
	if err != nil {
		return httpserver.FileContent{}, err
	}
	return httpserver.FileContent{
		AbsPath:     item.AbsPath,
		FileName:    item.FileName,
		ContentType: mediaContentType(item.MediaType, item.FileName),
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

func (p fileContentProvider) GetFilePreview(ctx context.Context, fileID int64) (httpserver.FileContent, error) {
	item, err := p.store.GetFilePreview(ctx, fileID)
	if err != nil {
		return httpserver.FileContent{}, err
	}
	return httpserver.FileContent{
		AbsPath:     item.AbsPath,
		FileName:    item.FileName,
		ContentType: mediaContentType(item.MediaType, item.AbsPath),
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

func (p fileContentProvider) GetVideoFramePreview(ctx context.Context, fileID int64, frameIndex int) (httpserver.FileContent, error) {
	item, err := p.store.GetVideoFramePreview(ctx, fileID, frameIndex)
	if err != nil {
		return httpserver.FileContent{}, err
	}
	return httpserver.FileContent{
		AbsPath:     item.AbsPath,
		FileName:    item.FileName,
		ContentType: mediaContentType(item.MediaType, item.AbsPath),
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

type fileTagCreator struct {
	service filetags.Service
}

func (p fileTagCreator) CreateFileTag(ctx context.Context, fileID int64, input httpserver.FileTagCreateRequest) error {
	return p.service.CreateFileTag(ctx, fileID, filetags.CreateInput{
		Namespace:   input.Namespace,
		Name:        input.Name,
		DisplayName: input.DisplayName,
	})
}

func (p fileTagCreator) DeleteFileTag(ctx context.Context, fileID int64, input httpserver.FileTagDeleteRequest) error {
	return p.service.DeleteFileTag(ctx, fileID, filetags.DeleteInput{
		Namespace: input.Namespace,
		Name:      input.Name,
	})
}

type fileJobRunner struct {
	understanding understandingJobEnqueuer
	embeddings    embeddingJobEnqueuer
	sameContent   sameContentJobEnqueuer
	sameSeries    sameSeriesJobEnqueuer
	samePerson    samePersonJobEnqueuer
}

func (p fileJobRunner) RecomputeFileEmbeddings(ctx context.Context, fileID int64, input httpserver.FileRecomputeRequest) error {
	if err := p.understanding.EnqueueUnderstanding(ctx, fileID); err != nil {
		return err
	}
	switch input.MediaType {
	case "image":
		return p.embeddings.EnqueueImageEmbedding(ctx, fileID)
	case "video":
		return p.embeddings.EnqueueVideoFrameEmbeddings(ctx, fileID)
	default:
		return fmt.Errorf("unsupported media type %s", input.MediaType)
	}
}

func (p fileJobRunner) ReclusterFile(ctx context.Context, fileID int64, _ httpserver.FileRecomputeRequest) error {
	if err := p.sameContent.EnqueueSameContent(ctx, fileID); err != nil {
		return err
	}
	if err := p.sameSeries.EnqueueSameSeries(ctx, fileID); err != nil {
		return err
	}
	return p.samePerson.EnqueueSamePerson(ctx, fileID)
}

type scannerJobEnqueuer struct {
	ensurer tasks.PostgresJobEnsurer
}

func (e scannerJobEnqueuer) EnqueueFileProcessing(ctx context.Context, fileID int64, mediaType string) error {
	jobType, err := extractionJobType(mediaType)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	if _, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "hash_file",
		Priority:   70,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	}); err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    jobType,
		Priority:   80,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

func extractionJobType(mediaType string) (string, error) {
	switch mediaType {
	case "image":
		return "extract_image_features", nil
	case "video":
		return "extract_video_features", nil
	default:
		return "", fmt.Errorf("unsupported media type %s", mediaType)
	}
}

type searchDocJobEnqueuer struct {
	ensurer tasks.PostgresJobEnsurer
}

func (e searchDocJobEnqueuer) EnqueueSearchDocument(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "recompute_search_doc",
		Priority:   90,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

type understandingJobEnqueuer struct {
	ensurer tasks.PostgresJobEnsurer
}

func (e understandingJobEnqueuer) EnqueueUnderstanding(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "infer_tags",
		Priority:   100,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

type qualityJobEnqueuer struct {
	ensurer tasks.PostgresJobEnsurer
}

func (e qualityJobEnqueuer) EnqueueQuality(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "infer_quality",
		Priority:   95,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

type embeddingJobEnqueuer struct {
	ensurer tasks.PostgresJobEnsurer
}

func (e embeddingJobEnqueuer) EnqueueImageEmbedding(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "embed_image",
		Priority:   98,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

func (e embeddingJobEnqueuer) EnqueueVideoFrameEmbeddings(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "embed_video_frames",
		Priority:   99,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

func (e embeddingJobEnqueuer) EnqueuePersonImageEmbedding(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "embed_person_image",
		Priority:   97,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

func (e embeddingJobEnqueuer) EnqueuePersonVideoFrameEmbeddings(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "embed_person_video_frames",
		Priority:   97,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

type sameContentJobEnqueuer struct {
	ensurer tasks.PostgresJobEnsurer
}

func (e sameContentJobEnqueuer) EnqueueSameContent(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "cluster_same_content",
		Priority:   110,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

type sameSeriesJobEnqueuer struct {
	ensurer tasks.PostgresJobEnsurer
}

func (e sameSeriesJobEnqueuer) EnqueueSameSeries(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "cluster_same_series",
		Priority:   115,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

type samePersonJobEnqueuer struct {
	ensurer tasks.PostgresJobEnsurer
}

func (e samePersonJobEnqueuer) EnqueueSamePerson(ctx context.Context, fileID int64) error {
	payload, err := json.Marshal(map[string]int64{
		"file_id": fileID,
	})
	if err != nil {
		return err
	}
	_, err = e.ensurer.EnsureJob(ctx, tasks.CreateJobInput{
		JobType:    "cluster_same_person",
		Priority:   118,
		TargetType: "file",
		TargetID:   fileID,
		Payload:    payload,
	})
	return err
}

func mediaContentType(mediaType string, fileName string) string {
	if value := mime.TypeByExtension(filepath.Ext(fileName)); value != "" {
		return value
	}
	switch mediaType {
	case "image":
		return "image/*"
	case "video":
		return "video/*"
	default:
		return "application/octet-stream"
	}
}
