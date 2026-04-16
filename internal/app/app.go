package app

import (
	"context"
	"net/http"

	"idea/internal/config"
	"idea/internal/httpserver"
	"idea/internal/systemstatus"
)

type Migrator interface {
	Run(ctx context.Context, dir string) error
}

type WorkerChecker interface {
	CheckWorkerHealth(ctx context.Context) error
}

type SystemStatusProvider interface {
	SystemStatus(ctx context.Context) (systemstatus.Snapshot, error)
}

type App struct {
	config           config.Config
	handler          http.Handler
	migrator         Migrator
	workerChecker    WorkerChecker
	systemStatus     SystemStatusProvider
	directoryPicker  httpserver.DirectoryPicker
	aiPromptSettings httpserver.AIPromptSettingsProvider
	taskSummary      httpserver.TaskSummaryProvider
	jobList          httpserver.JobListProvider
	jobCreator       httpserver.JobCreator
	jobRetrier       httpserver.JobRetrier
	jobEvents        httpserver.JobEventListProvider
	volumeList       httpserver.VolumeListProvider
	volumeCreator    httpserver.VolumeCreator
	volumeScanner    httpserver.VolumeScanner
	volumeDeleter    httpserver.VolumeDeleter
	tagList          httpserver.TagListProvider
	clusterList      httpserver.ClusterListProvider
	clusterDetail    httpserver.ClusterDetailProvider
	clusterSummary   httpserver.ClusterSummaryProvider
	clusterReview    httpserver.ClusterReviewer
	fileList         httpserver.FileListProvider
	fileDetail       httpserver.FileDetailProvider
	fileContent      httpserver.FileContentProvider
	fileTrasher      httpserver.FileTrasher
	fileRevealer     httpserver.FileRevealer
	fileOpener       httpserver.FileOpener
	fileReviewer     httpserver.FileReviewer
	fileTagCreator   httpserver.FileTagCreator
	fileJobRunner    httpserver.FileJobRunner
}

func New(cfg config.Config, migrator Migrator) *App {
	app := &App{
		config:   cfg,
		migrator: migrator,
	}
	app.refreshHandler()
	return app
}

func (a *App) Handler() http.Handler {
	return a.handler
}

func (a *App) Config() config.Config {
	return a.config
}

func (a *App) Bootstrap(ctx context.Context) error {
	if !a.config.RunMigrations {
		if a.workerChecker != nil {
			_ = a.workerChecker.CheckWorkerHealth(ctx)
		}
		return nil
	}
	if a.migrator == nil {
		if a.workerChecker != nil {
			_ = a.workerChecker.CheckWorkerHealth(ctx)
		}
		return nil
	}
	if err := a.migrator.Run(ctx, a.config.MigrationsDir); err != nil {
		return err
	}
	if a.workerChecker != nil {
		_ = a.workerChecker.CheckWorkerHealth(ctx)
	}
	return nil
}

func (a *App) SetWorkerChecker(checker WorkerChecker) {
	a.workerChecker = checker
}

func (a *App) SetSystemStatusProvider(provider SystemStatusProvider) {
	a.systemStatus = provider
	a.refreshHandler()
}

func (a *App) SetDirectoryPicker(provider httpserver.DirectoryPicker) {
	a.directoryPicker = provider
	a.refreshHandler()
}

func (a *App) SetAIPromptSettingsProvider(provider httpserver.AIPromptSettingsProvider) {
	a.aiPromptSettings = provider
	a.refreshHandler()
}

func (a *App) SetTaskSummaryProvider(provider httpserver.TaskSummaryProvider) {
	a.taskSummary = provider
	a.refreshHandler()
}

func (a *App) SetJobListProvider(provider httpserver.JobListProvider) {
	a.jobList = provider
	a.refreshHandler()
}

func (a *App) SetJobCreator(provider httpserver.JobCreator) {
	a.jobCreator = provider
	a.refreshHandler()
}

func (a *App) SetJobRetrier(provider httpserver.JobRetrier) {
	a.jobRetrier = provider
	a.refreshHandler()
}

func (a *App) SetJobEventListProvider(provider httpserver.JobEventListProvider) {
	a.jobEvents = provider
	a.refreshHandler()
}

func (a *App) SetVolumeListProvider(provider httpserver.VolumeListProvider) {
	a.volumeList = provider
	a.refreshHandler()
}

func (a *App) SetVolumeCreator(provider httpserver.VolumeCreator) {
	a.volumeCreator = provider
	a.refreshHandler()
}

func (a *App) SetVolumeScanner(provider httpserver.VolumeScanner) {
	a.volumeScanner = provider
	a.refreshHandler()
}

func (a *App) SetVolumeDeleter(provider httpserver.VolumeDeleter) {
	a.volumeDeleter = provider
	a.refreshHandler()
}

func (a *App) SetTagListProvider(provider httpserver.TagListProvider) {
	a.tagList = provider
	a.refreshHandler()
}

func (a *App) SetClusterListProvider(provider httpserver.ClusterListProvider) {
	a.clusterList = provider
	a.refreshHandler()
}

func (a *App) SetClusterDetailProvider(provider httpserver.ClusterDetailProvider) {
	a.clusterDetail = provider
	a.refreshHandler()
}

func (a *App) SetClusterSummaryProvider(provider httpserver.ClusterSummaryProvider) {
	a.clusterSummary = provider
	a.refreshHandler()
}

func (a *App) SetClusterReviewer(provider httpserver.ClusterReviewer) {
	a.clusterReview = provider
	a.refreshHandler()
}

func (a *App) SetFileListProvider(provider httpserver.FileListProvider) {
	a.fileList = provider
	a.refreshHandler()
}

func (a *App) SetFileDetailProvider(provider httpserver.FileDetailProvider) {
	a.fileDetail = provider
	a.refreshHandler()
}

func (a *App) SetFileContentProvider(provider httpserver.FileContentProvider) {
	a.fileContent = provider
	a.refreshHandler()
}

func (a *App) SetFileTrasher(provider httpserver.FileTrasher) {
	a.fileTrasher = provider
	a.refreshHandler()
}

func (a *App) SetFileRevealer(provider httpserver.FileRevealer) {
	a.fileRevealer = provider
	a.refreshHandler()
}

func (a *App) SetFileOpener(provider httpserver.FileOpener) {
	a.fileOpener = provider
	a.refreshHandler()
}

func (a *App) SetFileReviewer(provider httpserver.FileReviewer) {
	a.fileReviewer = provider
	a.refreshHandler()
}

func (a *App) SetFileTagCreator(provider httpserver.FileTagCreator) {
	a.fileTagCreator = provider
	a.refreshHandler()
}

func (a *App) SetFileJobRunner(provider httpserver.FileJobRunner) {
	a.fileJobRunner = provider
	a.refreshHandler()
}

func (a *App) refreshHandler() {
	a.handler = httpserver.NewMux(httpserver.Dependencies{
		FrontendDistDir:          a.config.FrontendDistDir,
		SystemStatusProvider:     a.systemStatus,
		DirectoryPicker:          a.directoryPicker,
		AIPromptSettingsProvider: a.aiPromptSettings,
		TaskSummaryProvider:      a.taskSummary,
		JobListProvider:          a.jobList,
		JobCreator:               a.jobCreator,
		JobRetrier:               a.jobRetrier,
		JobEventListProvider:     a.jobEvents,
		VolumeListProvider:       a.volumeList,
		VolumeCreator:            a.volumeCreator,
		VolumeScanner:            a.volumeScanner,
		VolumeDeleter:            a.volumeDeleter,
		TagListProvider:          a.tagList,
		ClusterListProvider:      a.clusterList,
		ClusterDetailProvider:    a.clusterDetail,
		ClusterSummaryProvider:   a.clusterSummary,
		ClusterReviewer:          a.clusterReview,
		FileListProvider:         a.fileList,
		FileDetailProvider:       a.fileDetail,
		FileContentProvider:      a.fileContent,
		FileTrasher:              a.fileTrasher,
		FileRevealer:             a.fileRevealer,
		FileOpener:               a.fileOpener,
		FileReviewer:             a.fileReviewer,
		FileTagCreator:           a.fileTagCreator,
		FileJobRunner:            a.fileJobRunner,
	})
}
