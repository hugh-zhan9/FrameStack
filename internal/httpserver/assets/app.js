let selectedJobID = null;
let selectedFileID = null;
let selectedClusterID = null;
let selectedFileIDs = new Set();
let currentClusters = [];
let refreshTimer = null;
let fileSearchQuery = '';
let fileSort = 'updated_desc';
let fileOffset = 0;
let hasMoreFiles = false;
let tagNamespace = '';

const FILES_PAGE_SIZE = 10;

let fileFilters = {
  mediaType: '',
  qualityTier: '',
  reviewAction: '',
  status: '',
  volumeID: '',
  tagNamespace: '',
  tag: '',
  clusterType: '',
  clusterStatus: '',
};

let clusterFilters = {
  clusterType: '',
  status: '',
};

async function loadSystemStatus() {
  const response = await fetch('/api/system-status');
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  const overallNode = document.querySelector('[data-system-overall]');
  const summaryNode = document.querySelector('[data-system-summary]');
  const banner = document.getElementById('systemStatusOverall');
  const checksNode = document.getElementById('systemStatusChecks');
  const checks = Array.isArray(payload.checks) ? payload.checks : [];
  const overallStatus = payload.status || 'unknown';

  if (overallNode) {
    overallNode.textContent = String(overallStatus);
  }
  if (summaryNode) {
    summaryNode.textContent = summarizeSystemStatus(overallStatus, checks);
  }
  if (banner) {
    banner.setAttribute('data-status', String(overallStatus));
  }
  if (checksNode) {
    checksNode.innerHTML = checks.map(renderSystemCheck).join('');
  }
}

async function loadTaskSummary() {
  const response = await fetch('/api/task-summary');
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  for (const key of ['pending', 'running', 'failed', 'dead', 'succeeded']) {
    const node = document.querySelector(`[data-key="${key}"]`);
    if (node) {
      node.textContent = String(payload[key] ?? 0);
    }
  }
}

async function loadClusterSummary() {
  const response = await fetch('/api/cluster-summary');
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  const items = Array.isArray(payload) ? payload : [];
  const summaryMap = new Map();
  for (const item of items) {
    if ((item.status || '') !== 'candidate') {
      continue;
    }
    summaryMap.set(String(item.cluster_type || ''), item);
  }
  for (const key of ['same_content', 'same_series', 'same_person']) {
    const summaryNode = document.querySelector(`[data-cluster-summary="${key}"]`);
    const membersNode = document.querySelector(`[data-cluster-members="${key}"]`);
    const item = summaryMap.get(key);
    if (summaryNode) {
      summaryNode.textContent = `${String(item?.cluster_count ?? 0)} groups`;
    }
    if (membersNode) {
      membersNode.textContent = `${String(item?.member_count ?? 0)} members`;
    }
  }
}

async function loadJobs() {
  const response = await fetch('/api/jobs?limit=8');
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  const list = document.getElementById('jobsList');
  if (!list) {
    return;
  }
  if (!Array.isArray(payload) || payload.length === 0) {
    list.innerHTML = '<li class="job-row empty">No jobs yet.</li>';
    selectedJobID = null;
    renderEvents([]);
    return;
  }
  if (!payload.some((job) => Number(job.id) === selectedJobID)) {
    selectedJobID = Number(payload[0].id);
  }
  list.innerHTML = payload.map((job) => renderJob(job, Number(job.id) === selectedJobID)).join('');
  bindJobSelection(payload);
  await loadJobEvents(selectedJobID);
}

async function loadVolumes() {
  const response = await fetch('/api/volumes');
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  const list = document.getElementById('volumesList');
  if (!list) {
    return;
  }
  if (!Array.isArray(payload) || payload.length === 0) {
    list.innerHTML = '<li class="volume-row empty">No volumes yet.</li>';
    renderVolumeFilter([]);
    return;
  }
  list.innerHTML = payload.map(renderVolume).join('');
  renderVolumeFilter(payload);
  bindVolumeActions();
}

async function loadTags() {
  const search = new URLSearchParams({ limit: '20' });
  if (tagNamespace) {
    search.set('namespace', tagNamespace);
  }
  const response = await fetch(`/api/tags?${search.toString()}`);
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  const list = document.getElementById('tagsList');
  if (!list) {
    return;
  }
  if (!Array.isArray(payload) || payload.length === 0) {
    list.innerHTML = '<li class="tag-cloud-item empty">No tags yet.</li>';
    return;
  }
  list.innerHTML = payload.map(renderTagCloudItem).join('');
  bindTagSelection();
}

async function loadClusters() {
  const search = new URLSearchParams({ limit: '8' });
  if (clusterFilters.clusterType) {
    search.set('cluster_type', clusterFilters.clusterType);
  }
  if (clusterFilters.status) {
    search.set('status', clusterFilters.status);
  }
  const response = await fetch(`/api/clusters?${search.toString()}`);
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  const list = document.getElementById('clustersList');
  if (!list) {
    return;
  }
  if (!Array.isArray(payload) || payload.length === 0) {
    currentClusters = [];
    list.innerHTML = '<li class="job-row empty">No clusters yet.</li>';
    selectedClusterID = null;
    renderClusterDetail(null);
    return;
  }
  currentClusters = payload;
  if (!payload.some((cluster) => Number(cluster.id) === selectedClusterID)) {
    selectedClusterID = Number(payload[0].id);
  }
  list.innerHTML = payload.map((cluster) => renderCluster(cluster, Number(cluster.id) === selectedClusterID)).join('');
  bindClusterSelection(payload);
  await loadClusterDetail(selectedClusterID);
}

async function loadFiles(append = false) {
  const search = new URLSearchParams({
    limit: String(append ? FILES_PAGE_SIZE : fileOffset + FILES_PAGE_SIZE),
    offset: String(append ? fileOffset : 0),
    sort: fileSort,
  });
  if (fileSearchQuery) {
    search.set('q', fileSearchQuery);
  }
  if (fileFilters.mediaType) {
    search.set('media_type', fileFilters.mediaType);
  }
  if (fileFilters.qualityTier) {
    search.set('quality_tier', fileFilters.qualityTier);
  }
  if (fileFilters.reviewAction) {
    search.set('review_action', fileFilters.reviewAction);
  }
  if (fileFilters.status) {
    search.set('status', fileFilters.status);
  }
  if (fileFilters.volumeID) {
    search.set('volume_id', fileFilters.volumeID);
  }
  if (fileFilters.tagNamespace) {
    search.set('tag_namespace', fileFilters.tagNamespace);
  }
  if (fileFilters.tag) {
    search.set('tag', fileFilters.tag);
  }
  if (fileFilters.clusterType) {
    search.set('cluster_type', fileFilters.clusterType);
  }
  if (fileFilters.clusterStatus) {
    search.set('cluster_status', fileFilters.clusterStatus);
  }

  const response = await fetch(`/api/files?${search.toString()}`);
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  const list = document.getElementById('filesList');
  const loadMoreButton = document.getElementById('filesLoadMoreButton');
  if (!list) {
    return;
  }
  if (!Array.isArray(payload) || payload.length === 0) {
    if (!append) {
      list.innerHTML = fileSearchQuery
        ? '<li class="file-row empty">No files matched the current search.</li>'
        : '<li class="file-row empty">No files indexed yet.</li>';
      selectedFileID = null;
      selectedFileIDs = new Set();
      renderFileDetail(null);
    }
    hasMoreFiles = false;
    updateLoadMoreButton(loadMoreButton);
    renderBulkActionBar();
    return;
  }

  if (!payload.some((file) => Number(file.id) === selectedFileID)) {
    selectedFileID = Number(payload[0].id);
  }
  if (!append) {
    const visibleIDs = new Set(payload.map((file) => Number(file.id)));
    selectedFileIDs = new Set([...selectedFileIDs].filter((id) => visibleIDs.has(id)));
  }
  const html = payload.map(renderFile).join('');
  if (append) {
    list.insertAdjacentHTML('beforeend', html);
  } else {
    list.innerHTML = html;
  }
  bindFileSelection(payload, append);
  await loadFileDetail(selectedFileID);
  hasMoreFiles = payload.length === FILES_PAGE_SIZE;
  updateLoadMoreButton(loadMoreButton);
  renderBulkActionBar();
}

async function loadFileDetail(fileID) {
  const meta = document.getElementById('fileDetailMeta');
  const container = document.getElementById('fileDetailContent');
  if (!meta || !container) {
    return;
  }
  if (!fileID) {
    renderFileDetail(null);
    return;
  }
  meta.textContent = `File #${fileID}`;
  const response = await fetch(`/api/files/${fileID}`);
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  renderFileDetail(payload);
}

async function loadClusterDetail(clusterID) {
  const meta = document.getElementById('clusterDetailMeta');
  const container = document.getElementById('clusterDetailContent');
  if (!meta || !container) {
    return;
  }
  if (!clusterID) {
    renderClusterDetail(null);
    return;
  }
  meta.textContent = `Cluster #${clusterID}`;
  const response = await fetch(`/api/clusters/${clusterID}`);
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  renderClusterDetail(payload);
}

async function loadJobEvents(jobID) {
  const list = document.getElementById('jobEventsList');
  const meta = document.getElementById('timelineMeta');
  if (!list || !meta) {
    return;
  }
  if (!jobID) {
    meta.textContent = 'Select a job to inspect recent events';
    renderEvents([]);
    return;
  }
  meta.textContent = `Recent events for job #${jobID}`;
  const response = await fetch(`/api/jobs/${jobID}/events?limit=8`);
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const payload = await response.json();
  renderEvents(payload);
}

function renderJob(job, isSelected) {
  const target = job.target_type ? `${job.target_type} #${job.target_id || '-'}` : 'system';
  const stage = job.progress_stage || 'idle';
  const attempts = `${job.attempt_count ?? 0}/${job.max_attempts ?? 0}`;
  const retryAction = job.status === 'failed' || job.status === 'dead'
    ? `<div class="job-actions"><button class="retry-button" type="button" data-retry-job-id="${escapeHTML(job.id || '')}">Retry</button></div>`
    : '';
  const errorLine = job.last_error ? `<p class="job-error">${escapeHTML(job.last_error)}</p>` : '';
  return `
    <li class="job-row" data-job-id="${escapeHTML(job.id || '')}" data-selected="${isSelected ? 'true' : 'false'}">
      <div class="job-main">
        <p class="job-title">${escapeHTML(job.job_type || 'unknown')}</p>
        <p class="job-subtitle">${escapeHTML(target)}</p>
        ${errorLine}
      </div>
      <div class="job-status">Status<strong>${escapeHTML(job.status || 'unknown')}</strong></div>
      <div class="job-stage">Stage<strong>${escapeHTML(stage)}</strong></div>
      <div class="job-attempts">Attempts<strong>${escapeHTML(attempts)}</strong>${retryAction}</div>
    </li>
  `;
}

function renderSystemCheck(item) {
  return `
    <div class="stat-card status-card" data-status="${escapeHTML(item.status || 'unknown')}">
      <span class="label">${escapeHTML(item.name || 'unknown')}</span>
      <strong>${escapeHTML(item.status || 'unknown')}</strong>
      <small>${escapeHTML(item.message || 'ready')}</small>
    </div>
  `;
}

function renderVolume(volume) {
  const status = volume.is_online ? 'online' : 'offline';
  return `
    <li class="volume-row">
      <div>
        <p class="volume-title">${escapeHTML(volume.display_name || 'Unnamed Volume')}</p>
        <p class="volume-subtitle">${escapeHTML(volume.mount_path || '')}</p>
      </div>
      <div class="volume-status">Status<strong>${escapeHTML(status)}</strong></div>
      <div class="volume-actions">
        <button class="retry-button" type="button" data-scan-volume-id="${escapeHTML(volume.id || '')}">Scan</button>
      </div>
    </li>
  `;
}

function renderCluster(cluster, isSelected) {
  const confidence = typeof cluster.confidence === 'number' ? cluster.confidence.toFixed(2) : 'n/a';
  const queueLabel = currentClusters.length > 0
    ? `${currentClusters.findIndex((item) => Number(item.id) === Number(cluster.id)) + 1}/${currentClusters.length}`
    : 'n/a';
  const memberSummary = cluster.cluster_type === 'same_person'
    ? `${String(cluster.strong_member_count || 0)} strong • ${escapeClusterEvidenceSummary(cluster)}`
    : cluster.cluster_type === 'same_content'
      ? 'best pick ready'
      : `${String(cluster.member_count || 0)} members`;
  const scoreSummary = typeof cluster.top_member_score === 'number'
    ? cluster.top_member_score.toFixed(2)
    : confidence;
  return `
    <li class="job-row" data-cluster-id="${escapeHTML(cluster.id || '')}" data-selected="${isSelected ? 'true' : 'false'}">
      <div class="job-main">
        <div class="cluster-row-head">
          <p class="job-title">${escapeHTML(cluster.title || cluster.cluster_type || 'cluster')}</p>
          <span class="cluster-row-index">${escapeHTML(queueLabel)}</span>
        </div>
        <p class="job-subtitle">${escapeHTML(cluster.cluster_type || 'unknown')} • ${escapeHTML(cluster.status || 'unknown')}</p>
      </div>
      <div class="job-status">Summary<strong>${escapeHTML(memberSummary)}</strong></div>
      <div class="job-stage">${cluster.cluster_type === 'same_content' ? 'Pick Score' : 'Top Score'}<strong>${escapeHTML(scoreSummary)}</strong></div>
    </li>
  `;
}

function escapeClusterEvidenceSummary(cluster) {
  const personVisualCount = Number(cluster.person_visual_count || 0);
  const genericVisualCount = Number(cluster.generic_visual_count || 0);
  const topEvidenceType = cluster.top_evidence_type || '';
  if (personVisualCount > 0) {
    return `PV ${personVisualCount}`;
  }
  if (genericVisualCount > 0) {
    return `GF ${genericVisualCount}`;
  }
  if (topEvidenceType === 'generic_visual') {
    return 'fallback only';
  }
  return 'no evidence';
}

function renderTagCloudItem(tag) {
  const value = tag.display_name || tag.name || '';
  const namespace = tag.namespace || '';
  const active = fileFilters.tag && fileFilters.tag === value;
  return `
    <li class="tag-cloud-item" data-selected="${active ? 'true' : 'false'}">
      <button class="tag-cloud-button" type="button" data-tag-value="${escapeHTML(value)}" data-tag-namespace="${escapeHTML(namespace)}">
        <strong>${escapeHTML(value || 'unknown')}</strong>
        <span>${escapeHTML(namespace || 'unknown')} • ${escapeHTML(String(tag.file_count || 0))}</span>
      </button>
    </li>
  `;
}

function renderFile(file) {
  const details = [];
  if (file.width && file.height) {
    details.push(`${file.width}x${file.height}`);
  }
  if (file.duration_ms) {
    details.push(formatDuration(file.duration_ms));
  }
  if (file.format) {
    details.push(file.format);
  } else if (file.container) {
    details.push(file.container);
  }
  const detailText = details.length > 0 ? details.join(' • ') : 'metadata pending';
  const tagNames = Array.isArray(file.tag_names) ? file.tag_names : [];
  const tagSummary = tagNames.length > 0
    ? `<p class="file-tags">${tagNames.slice(0, 4).map((tag) => `<span>${escapeHTML(tag)}</span>`).join('')}</p>`
    : '';
  const statPills = [
    { label: 'Type', value: file.media_type || 'unknown' },
    { label: 'Quality', value: formatQuality(file.quality_tier, file.quality_score) },
    { label: 'Review', value: file.review_action || 'none' },
  ];
  const previewHTML = file.has_preview
    ? `
      <div class="file-card-preview">
        <img class="file-card-preview-media" src="/api/files/${encodeURIComponent(String(file.id || ''))}/preview" alt="${escapeHTML(file.file_name || 'preview')}" loading="lazy" />
        ${file.media_type === 'video' ? '<span class="file-card-preview-badge">Video</span>' : ''}
      </div>
    `
    : `<div class="file-card-preview file-card-preview-empty">${escapeHTML((file.media_type || 'file').toUpperCase())}</div>`;
  return `
    <li class="file-row" data-file-id="${escapeHTML(file.id || '')}" data-selected="${Number(file.id) === selectedFileID ? 'true' : 'false'}" data-bulk-selected="${selectedFileIDs.has(Number(file.id)) ? 'true' : 'false'}">
      ${previewHTML}
      <div class="file-card-body">
        <div class="file-card-header">
          <div class="file-card-title-wrap">
            <input class="file-card-checkbox" type="checkbox" data-file-select-id="${escapeHTML(file.id || '')}" ${selectedFileIDs.has(Number(file.id)) ? 'checked' : ''} aria-label="Select file ${escapeHTML(file.file_name || 'unknown')}" />
            <p class="file-title">${escapeHTML(file.file_name || 'unknown')}</p>
          </div>
          <span class="file-card-status">${escapeHTML(file.status || 'unknown')}</span>
        </div>
        <p class="file-subtitle">${escapeHTML(file.abs_path || '')}</p>
        <p class="file-card-detail">${escapeHTML(detailText)}</p>
        ${tagSummary}
        <div class="file-card-stats">
          ${statPills.map((item) => `
            <div class="file-card-stat">
              <span>${escapeHTML(item.label)}</span>
              <strong>${escapeHTML(item.value)}</strong>
            </div>
          `).join('')}
        </div>
      </div>
    </li>
  `;
}

function renderFileDetail(file) {
  const meta = document.getElementById('fileDetailMeta');
  const container = document.getElementById('fileDetailContent');
  if (!meta || !container) {
    return;
  }
  if (!file) {
    meta.textContent = 'Select a file to inspect detail and path history';
    container.className = 'file-detail-empty';
    container.textContent = 'No file selected.';
    return;
  }

  const detailItems = [
    ['Name', file.file_name || 'unknown'],
    ['Path', file.abs_path || ''],
    ['Media', file.media_type || 'unknown'],
    ['Status', file.status || 'unknown'],
    ['Quality', formatQuality(file.quality_tier, file.quality_score)],
    ['Review', file.review_action || 'none'],
    ['Format', file.format || file.container || ''],
    ['Size', String(file.size_bytes || 0)],
  ];
  if (file.width && file.height) {
    detailItems.push(['Resolution', `${file.width}x${file.height}`]);
  }
  if (file.duration_ms) {
    detailItems.push(['Duration', formatDuration(file.duration_ms)]);
  }
  if (file.media_type === 'video') {
    if (typeof file.fps === 'number') {
      detailItems.push(['FPS', Number(file.fps).toFixed(2)]);
    }
    if (typeof file.bitrate === 'number') {
      detailItems.push(['Bitrate', formatBitrate(file.bitrate)]);
    }
    if (file.video_codec) {
      detailItems.push(['Video Codec', file.video_codec]);
    }
    if (file.audio_codec) {
      detailItems.push(['Audio Codec', file.audio_codec]);
    }
  }

  const history = Array.isArray(file.path_history) ? file.path_history : [];
  const analyses = Array.isArray(file.current_analyses) ? file.current_analyses : [];
  const tags = Array.isArray(file.tags) ? file.tags : [];
  const reviewActions = Array.isArray(file.review_actions) ? file.review_actions : [];
  const clusters = Array.isArray(file.clusters) ? file.clusters : [];
  const embeddings = Array.isArray(file.embeddings) ? file.embeddings : [];
  const videoFrames = Array.isArray(file.video_frames) ? file.video_frames : [];

  const analysisHTML = analyses.length === 0
    ? '<li class="analysis-row"><p class="analysis-title">No current analyses yet.</p></li>'
    : analyses.map((item) => {
        const metaParts = [item.status || 'unknown'];
        const qualityText = formatQuality(item.quality_tier, item.quality_score);
        if (qualityText !== 'pending') {
          metaParts.push(qualityText);
        }
        metaParts.push(formatTimestamp(item.created_at));
        return `
          <li class="analysis-row">
            <p class="analysis-title">${escapeHTML(item.analysis_type || 'unknown')} · ${escapeHTML(item.summary || '')}</p>
            <p class="analysis-meta">${escapeHTML(metaParts.join(' • '))}</p>
          </li>
        `;
      }).join('');

  const tagsHTML = tags.length === 0
    ? '<li class="tag-chip empty">No tags yet.</li>'
    : tags.map((item) => `
        <li class="tag-chip">
          <div class="tag-chip-main">
            <strong>${escapeHTML(item.display_name || item.name || 'unknown')}</strong>
            <span>${escapeHTML(item.namespace || 'unknown')} • ${escapeHTML(item.source || 'unknown')}</span>
          </div>
          ${item.source === 'human'
            ? `<button class="tag-chip-delete" type="button" data-delete-tag-namespace="${escapeHTML(item.namespace || '')}" data-delete-tag-name="${escapeHTML(item.name || '')}">Remove</button>`
            : ''
          }
        </li>
      `).join('');

  const reviewHTML = reviewActions.length === 0
    ? '<li class="history-row"><p class="history-path">No review actions yet.</p></li>'
    : reviewActions.map((item) => `
        <li class="history-row">
          <p class="history-path">${escapeHTML(item.action_type || 'unknown')}</p>
          <p class="history-meta">${escapeHTML(item.note || 'no note')} • ${escapeHTML(formatTimestamp(item.created_at))}</p>
        </li>
      `).join('');

  const historyHTML = history.length === 0
    ? '<li class="history-row"><p class="history-path">No path history yet.</p></li>'
    : history.map((item) => `
        <li class="history-row">
          <p class="history-path">${escapeHTML(item.abs_path || '')}</p>
          <p class="history-meta">${escapeHTML(item.event_type || 'unknown')} • ${escapeHTML(formatTimestamp(item.seen_at))}</p>
        </li>
      `).join('');

  const clustersHTML = clusters.length === 0
    ? '<li class="history-row"><p class="history-path">No cluster memberships yet.</p></li>'
    : clusters.map((item) => `
        <li class="history-row" data-open-cluster-id="${escapeHTML(item.id || '')}" data-open-cluster-type="${escapeHTML(item.cluster_type || '')}">
          <p class="history-path">${escapeHTML(item.title || item.cluster_type || 'cluster')}</p>
          <p class="history-meta">${escapeHTML(item.cluster_type || 'unknown')} • ${escapeHTML(item.status || 'unknown')}</p>
        </li>
      `).join('');

  const embeddingsHTML = embeddings.length === 0
    ? '<li class="analysis-row"><p class="analysis-title">No embeddings yet.</p></li>'
    : embeddings.map((item) => `
        <li class="analysis-row">
          <p class="analysis-title">${escapeHTML(item.embedding_type || 'unknown')} · ${escapeHTML(item.provider || 'unknown')}</p>
          <p class="analysis-meta">${escapeHTML(item.model_name || 'unknown')} • ${escapeHTML(String(item.vector_count || 0))} vectors</p>
        </li>
      `).join('');

  const videoFramesHTML = videoFrames.length === 0
    ? '<li class="analysis-row"><p class="analysis-title">No keyframes recorded yet.</p></li>'
    : videoFrames.map((item, index) => `
        <li class="analysis-row">
          <div class="analysis-frame-row">
            <img
              class="analysis-frame-preview"
              src="/api/files/${encodeURIComponent(String(file.id || ''))}/frames/${index}/preview"
              alt="${escapeHTML((file.file_name || 'video') + ' frame ' + String(index + 1))}"
              loading="lazy"
            />
            <div class="analysis-frame-copy">
              <p class="analysis-title">${escapeHTML(item.frame_role || 'frame')} · ${escapeHTML(formatDuration(item.timestamp_ms || 0))}</p>
              <p class="analysis-meta">${escapeHTML(item.phash || 'phash pending')}</p>
            </div>
          </div>
        </li>
      `).join('');

  const previewHTML = renderFilePreview(file);

  meta.textContent = `File #${file.id}`;
  container.className = '';
  container.innerHTML = `
    ${previewHTML}
    <div class="file-detail-actions">
      <button id="revealFileButton" type="button">Reveal In Finder</button>
      <button id="recomputeEmbeddingsButton" type="button">Refresh AI + Embeddings</button>
      <button id="reclusterFileButton" type="button">Recluster</button>
      <button id="keepFileButton" type="button">Keep</button>
      <button id="favoriteFileButton" type="button">Favorite</button>
      <button id="trashFileButton" class="danger-button" type="button">Move To Trash</button>
    </div>
    <form id="fileTagForm" class="inline-tag-form">
      <select id="manualTagNamespace" name="namespace">
        <option value="content">content</option>
        <option value="quality">quality</option>
        <option value="management">management</option>
        <option value="sensitive">sensitive</option>
        <option value="person">person</option>
      </select>
      <input id="manualTagName" name="name" type="text" placeholder="tag name" />
      <input id="manualTagDisplayName" name="display_name" type="text" placeholder="display name (optional)" />
      <button type="submit">Add Tag</button>
    </form>
    <div class="file-detail-grid">
      ${detailItems.map(([label, value]) => `
        <div class="file-detail-card">
          <span>${escapeHTML(label)}</span>
          <strong>${escapeHTML(value)}</strong>
        </div>
      `).join('')}
    </div>
    <ul class="analysis-list">${embeddingsHTML}</ul>
    ${file.media_type === 'video' ? `<ul class="analysis-list">${videoFramesHTML}</ul>` : ''}
    <ul class="analysis-list">${analysisHTML}</ul>
    <ul class="tag-list">${tagsHTML}</ul>
    <ul class="history-list">${clustersHTML}</ul>
    <ul class="history-list">${reviewHTML}</ul>
    <ul class="history-list">${historyHTML}</ul>
  `;
  bindFileDetailActions(file.id);
  bindFileClusterLinks();
}

function renderFilePreview(file) {
  const contentURL = `/api/files/${encodeURIComponent(String(file.id || ''))}/content`;
  if (file.media_type === 'image') {
    return `
      <div class="file-preview">
        <img class="file-preview-media" src="${contentURL}" alt="${escapeHTML(file.file_name || 'image preview')}" loading="lazy" />
      </div>
    `;
  }
  if (file.media_type === 'video') {
    return `
      <div class="file-preview">
        <video class="file-preview-media" src="${contentURL}" controls preload="metadata"></video>
      </div>
    `;
  }
  return '';
}

function renderClusterDetail(cluster) {
  const meta = document.getElementById('clusterDetailMeta');
  const container = document.getElementById('clusterDetailContent');
  if (!meta || !container) {
    return;
  }
  if (!cluster) {
    meta.textContent = 'Select a cluster to inspect members';
    container.className = 'file-detail-empty';
    container.textContent = 'No cluster selected.';
    return;
  }
  const detailItems = [
    ['Title', cluster.title || cluster.cluster_type || 'cluster'],
    ['Type', cluster.cluster_type || 'unknown'],
    ['Status', cluster.status || 'unknown'],
    ['Members', String(cluster.member_count || 0)],
    ['Strong', String(cluster.strong_member_count || 0)],
    ['Top Score', typeof cluster.top_member_score === 'number' ? cluster.top_member_score.toFixed(2) : 'n/a'],
    ['Confidence', typeof cluster.confidence === 'number' ? cluster.confidence.toFixed(2) : 'n/a'],
  ];
  const members = Array.isArray(cluster.members) ? cluster.members : [];
  const clusterPosition = summarizeClusterPosition(cluster.id);
  const previousCluster = getAdjacentCluster(cluster.id, -1);
  const nextCluster = getAdjacentCluster(cluster.id, 1);
  const bestQualityMember = members.find((item) => item.role === 'best_quality') || null;
  const seriesFocusMember = members.find((item) => item.role === 'series_focus') || null;
  const personVisualCount = Number(cluster.person_visual_count || 0);
  const genericVisualCount = Number(cluster.generic_visual_count || 0);
  const topEvidenceType = cluster.top_evidence_type || '';
  const roles = members.reduce((counts, item) => {
    const key = item.role || 'member';
    counts[key] = (counts[key] || 0) + 1;
    return counts;
  }, {});
  const roleSummary = Object.keys(roles).length === 0
    ? 'No roles yet'
    : Object.entries(roles).map(([key, value]) => `${key}: ${value}`).join(' • ');
  const membersHTML = members.length === 0
    ? '<li class="cluster-member-card empty"><p class="history-path">No cluster members yet.</p></li>'
    : members.map((item) => {
        const metaParts = [
          item.media_type || 'unknown',
          formatQuality(item.quality_tier, item.score),
        ];
        if (item.embedding_provider || item.embedding_model) {
          metaParts.push(`${item.embedding_type || 'unknown'} • ${item.embedding_provider || 'unknown'} • ${item.embedding_model || 'unknown'} • ${String(item.embedding_vector_count || 0)} vectors`);
        }
        const evidenceHint = cluster.cluster_type === 'same_person'
          ? formatPersonEvidence(item.embedding_type)
          : '';
        const evidenceTrail = cluster.cluster_type === 'same_person'
          ? summarizePersonEvidenceTrail(item, members[0])
          : '';
        const previewHTML = `
          <div class="cluster-member-preview">
            <img
              class="cluster-member-preview-media"
              src="/api/files/${encodeURIComponent(String(item.file_id || ''))}/preview"
              alt="${escapeHTML(item.file_name || 'cluster member preview')}"
              loading="lazy"
            />
            ${item.media_type === 'video' ? '<span class="cluster-member-preview-badge">Video</span>' : ''}
          </div>
        `;
        return `
          <li class="cluster-member-card" data-open-file-id="${escapeHTML(item.file_id || '')}">
            ${previewHTML}
            <div class="cluster-member-card-head">
              <span class="cluster-member-role" data-role="${escapeHTML(item.role || 'member')}">${escapeHTML(formatClusterMemberRole(item.role))}</span>
              <span class="cluster-member-id">#${escapeHTML(item.file_id || '')}</span>
            </div>
            <div class="cluster-member-strength" data-strength="${escapeHTML(classifyCandidateStrength(item.score))}">
              ${escapeHTML(formatCandidateStrength(item.score))}
            </div>
            ${evidenceHint ? `<div class="cluster-member-evidence">${escapeHTML(evidenceHint)}</div>` : ''}
            ${evidenceTrail ? `<div class="cluster-member-evidence cluster-member-evidence-secondary">${escapeHTML(evidenceTrail)}</div>` : ''}
            <p class="history-path">${escapeHTML(item.file_name || 'unknown')}</p>
            <p class="history-meta">${escapeHTML(metaParts.join(' • '))}</p>
          </li>
        `;
      }).join('');
  meta.textContent = `Cluster #${cluster.id}`;
  container.className = '';
  container.innerHTML = `
    <div class="file-detail-actions">
      <button id="previousClusterButton" type="button" ${previousCluster ? '' : 'disabled'}>Previous</button>
      <button id="nextClusterButton" type="button" ${nextCluster ? '' : 'disabled'}>Next</button>
      <button id="confirmClusterButton" type="button">Confirm Group</button>
      <button id="ignoreClusterButton" type="button">Ignore Group</button>
      <button id="resetClusterButton" type="button">Reset Candidate</button>
      <button id="keepClusterButton" type="button">Keep Group</button>
      <button id="favoriteClusterButton" type="button">Favorite Group</button>
      <button id="trashCandidateClusterButton" class="danger-button" type="button">Mark Trash Candidate</button>
    </div>
    <div class="file-detail-grid">
      ${detailItems.map(([label, value]) => `
        <div class="file-detail-card">
          <span>${escapeHTML(label)}</span>
          <strong>${escapeHTML(value)}</strong>
        </div>
      `).join('')}
    </div>
    <div class="cluster-detail-summary">
      <div class="cluster-summary-chip">
        <span>Queue Position</span>
        <strong>${escapeHTML(clusterPosition)}</strong>
      </div>
      ${bestQualityMember && cluster.cluster_type === 'same_content' ? `
        <div class="cluster-summary-chip cluster-summary-chip-accent">
          <span>Recommended Keep</span>
          <strong>${escapeHTML(bestQualityMember.file_name || `#${bestQualityMember.file_id}`)}</strong>
        </div>
      ` : ''}
      ${seriesFocusMember && cluster.cluster_type === 'same_series' ? `
        <div class="cluster-summary-chip cluster-summary-chip-accent">
          <span>Review Focus</span>
          <strong>${escapeHTML(seriesFocusMember.file_name || `#${seriesFocusMember.file_id}`)}</strong>
        </div>
      ` : ''}
      ${cluster.cluster_type === 'same_person' ? `
        <div class="cluster-summary-chip cluster-summary-chip-accent">
          <span>Person Visual</span>
          <strong>${escapeHTML(String(personVisualCount))} members</strong>
        </div>
        <div class="cluster-summary-chip">
          <span>Generic Fallback</span>
          <strong>${escapeHTML(String(genericVisualCount))} members</strong>
        </div>
        <div class="cluster-summary-chip">
          <span>Top Evidence</span>
          <strong>${escapeHTML(formatPersonEvidence(topEvidenceType) || 'n/a')}</strong>
        </div>
      ` : ''}
      <div class="cluster-summary-chip">
        <span>Role Mix</span>
        <strong>${escapeHTML(roleSummary)}</strong>
      </div>
      <div class="cluster-summary-chip">
        <span>Decision Hint</span>
        <strong>${escapeHTML(summarizeClusterHint(cluster))}</strong>
      </div>
    </div>
    <ul class="cluster-member-grid">${membersHTML}</ul>
  `;
  bindClusterDetailActions(cluster.id);
  bindClusterMemberLinks();
}

function summarizeClusterHint(cluster) {
  const members = Array.isArray(cluster.members) ? cluster.members : [];
  if (members.length === 0) {
    return 'No members to review';
  }
  if (cluster.cluster_type === 'same_content') {
    const bestQualityMember = members.find((item) => item.role === 'best_quality');
    if (bestQualityMember) {
      return `Keep ${bestQualityMember.file_name || `#${bestQualityMember.file_id}`} first`;
    }
  }
  if (cluster.cluster_type === 'same_series') {
    const seriesFocusMember = members.find((item) => item.role === 'series_focus');
    if (seriesFocusMember) {
      return `Start with ${seriesFocusMember.file_name || `#${seriesFocusMember.file_id}`}`;
    }
  }
  if (cluster.cluster_type === 'same_person') {
    const personVisualCount = Number(cluster.person_visual_count || 0);
    if (personVisualCount > 0) {
      return `Review person-vector matches first • ${personVisualCount} strong members`;
    }
    const genericVisualCount = Number(cluster.generic_visual_count || 0);
    if (genericVisualCount > 0) {
      return `Fallback visual evidence only • ${genericVisualCount} members`;
    }
  }
  const topQuality = members
    .map((item) => formatQuality(item.quality_tier, item.score))
    .filter((item) => item && item !== 'pending')
    .slice(0, 3);
  if (topQuality.length === 0) {
    return 'Review media preview and tags first';
  }
  return `Compare quality first • ${topQuality.join(' / ')}`;
}

function formatClusterMemberRole(role) {
  switch (role) {
    case 'best_quality':
      return 'Best Quality';
    case 'duplicate_candidate':
      return 'Duplicate';
    case 'series_focus':
      return 'Focus';
    case 'cover':
      return 'Cover';
    case 'member':
      return 'Member';
    default:
      return role || 'member';
  }
}

function classifyCandidateStrength(score) {
  if (typeof score !== 'number') {
    return 'unknown';
  }
  if (score >= 0.9) {
    return 'strong';
  }
  if (score >= 0.75) {
    return 'medium';
  }
  return 'weak';
}

function formatCandidateStrength(score) {
  if (typeof score !== 'number') {
    return 'Strength pending';
  }
  const level = classifyCandidateStrength(score);
  if (level === 'strong') {
    return `Strong candidate • ${score.toFixed(2)}`;
  }
  if (level === 'medium') {
    return `Medium candidate • ${score.toFixed(2)}`;
  }
  return `Weak candidate • ${score.toFixed(2)}`;
}

function formatPersonEvidence(embeddingType) {
  if (embeddingType === 'person_visual') {
    return 'Person vector evidence';
  }
  if (embeddingType === 'image_visual' || embeddingType === 'video_frame_visual') {
    return 'Generic visual fallback';
  }
  return '';
}

function summarizePersonEvidenceTrail(item, anchor) {
  if (!item || !anchor || item.file_id === anchor.file_id) {
    return '';
  }
  const hints = [];
  if (item.has_face) {
    hints.push('has face');
  }
  if (normalizedPersonValue(item.subject_count) && normalizedPersonValue(item.subject_count) === normalizedPersonValue(anchor.subject_count)) {
    hints.push(`subject ${normalizedPersonValue(item.subject_count)}`);
  }
  if (normalizedPersonValue(item.capture_type) && normalizedPersonValue(item.capture_type) === normalizedPersonValue(anchor.capture_type)) {
    hints.push(`capture ${normalizedPersonValue(item.capture_type)}`);
  }
  if (sameFileFamily(item.file_name, anchor.file_name)) {
    hints.push('same family');
  }
  if (sameParentFolder(item.abs_path, anchor.abs_path)) {
    hints.push('same folder');
  }
  return hints.slice(0, 3).join(' • ');
}

function normalizedPersonValue(value) {
  return String(value || '').trim().toLowerCase();
}

function sameFileFamily(left, right) {
  return normalizedFileFamily(left) && normalizedFileFamily(left) === normalizedFileFamily(right);
}

function normalizedFileFamily(fileName) {
  const base = String(fileName || '').trim().toLowerCase().replace(/\.[^/.]+$/, '').replace(/^[-_.\s]+|[-_.\s]+$/g, '');
  if (!base) {
    return '';
  }
  return base.replace(/[-_.\s]*\d+$/, '').replace(/^[-_.\s]+|[-_.\s]+$/g, '');
}

function sameParentFolder(leftPath, rightPath) {
  const left = parentFolder(leftPath);
  const right = parentFolder(rightPath);
  return left && right && left === right;
}

function parentFolder(pathValue) {
  const value = String(pathValue || '');
  const index = value.lastIndexOf('/');
  if (index <= 0) {
    return '';
  }
  return value.slice(0, index);
}

function getClusterIndex(clusterID) {
  return currentClusters.findIndex((cluster) => Number(cluster.id) === Number(clusterID));
}

function getAdjacentCluster(clusterID, direction) {
  const index = getClusterIndex(clusterID);
  if (index < 0) {
    return null;
  }
  const targetIndex = index + direction;
  if (targetIndex < 0 || targetIndex >= currentClusters.length) {
    return null;
  }
  return currentClusters[targetIndex];
}

function summarizeClusterPosition(clusterID) {
  const index = getClusterIndex(clusterID);
  if (index < 0 || currentClusters.length === 0) {
    return 'Not in current view';
  }
  return `${index + 1} / ${currentClusters.length}`;
}

async function selectCluster(clusterID) {
  if (!clusterID) {
    return;
  }
  selectedClusterID = Number(clusterID);
  const list = document.getElementById('clustersList');
  if (list && Array.isArray(currentClusters) && currentClusters.length > 0) {
    list.innerHTML = currentClusters
      .map((cluster) => renderCluster(cluster, Number(cluster.id) === selectedClusterID))
      .join('');
    bindClusterSelection(currentClusters);
    const selectedNode = list.querySelector(`[data-cluster-id="${CSS.escape(String(selectedClusterID))}"]`);
    if (selectedNode) {
      selectedNode.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }
  }
  await loadClusterDetail(selectedClusterID);
}

function getAutoAdvanceClusterID(clusterID) {
  const nextCluster = getAdjacentCluster(clusterID, 1);
  if (nextCluster) {
    return Number(nextCluster.id);
  }
  const previousCluster = getAdjacentCluster(clusterID, -1);
  if (previousCluster) {
    return Number(previousCluster.id);
  }
  return Number(clusterID);
}

async function advanceAfterClusterAction(clusterID, action) {
  selectedClusterID = action === 'stay'
    ? Number(clusterID)
    : getAutoAdvanceClusterID(clusterID);
  await refresh();
}

function isEditableTarget(target) {
  if (!target) {
    return false;
  }
  const tagName = String(target.tagName || '').toLowerCase();
  if (tagName === 'input' || tagName === 'textarea' || tagName === 'select' || tagName === 'button') {
    return true;
  }
  return Boolean(target.closest('input, textarea, select, button, [contenteditable="true"]'));
}

function bindKeyboardShortcuts() {
  document.addEventListener('keydown', async (event) => {
    if (isEditableTarget(event.target)) {
      return;
    }
    if (!selectedClusterID || currentClusters.length === 0) {
      return;
    }
    try {
      if (event.key === 'ArrowDown' || event.key === 'j') {
        event.preventDefault();
        const nextCluster = getAdjacentCluster(selectedClusterID, 1);
        if (nextCluster) {
          await selectCluster(nextCluster.id);
        }
        return;
      }
      if (event.key === 'ArrowUp' || event.key === 'k') {
        event.preventDefault();
        const previousCluster = getAdjacentCluster(selectedClusterID, -1);
        if (previousCluster) {
          await selectCluster(previousCluster.id);
        }
      }
    } catch (error) {
      console.error(error);
    }
  });
}

function renderVolumeFilter(volumes) {
  const select = document.getElementById('fileVolumeFilter');
  if (!select) {
    return;
  }
  const currentValue = fileFilters.volumeID;
  const options = ['<option value="">All Volumes</option>'];
  for (const volume of volumes) {
    const id = String(volume.id || '');
    const selected = currentValue === id ? ' selected' : '';
    options.push(`<option value="${escapeHTML(id)}"${selected}>${escapeHTML(volume.display_name || volume.mount_path || id)}</option>`);
  }
  select.innerHTML = options.join('');
}

function updateLoadMoreButton(button) {
  if (!button) {
    return;
  }
  button.disabled = !hasMoreFiles;
  button.textContent = hasMoreFiles ? 'Load More' : 'No More Files';
}

function bindJobSelection(payload) {
  for (const node of document.querySelectorAll('[data-job-id]')) {
    node.addEventListener('click', async () => {
      selectedJobID = Number(node.getAttribute('data-job-id'));
      const list = document.getElementById('jobsList');
      if (list) {
        list.innerHTML = payload.map((job) => renderJob(job, Number(job.id) === selectedJobID)).join('');
        bindJobSelection(payload);
      }
      try {
        await loadJobEvents(selectedJobID);
      } catch (error) {
        console.error(error);
      }
    });
  }

  for (const button of document.querySelectorAll('[data-retry-job-id]')) {
    button.addEventListener('click', async (event) => {
      event.stopPropagation();
      const jobID = Number(button.getAttribute('data-retry-job-id'));
      if (!jobID) {
        return;
      }
      try {
        const response = await fetch(`/api/jobs/${jobID}/retry`, { method: 'POST' });
        if (!response.ok) {
          throw new Error(`retry failed: ${response.status}`);
        }
        await refresh();
      } catch (error) {
        console.error(error);
      }
    });
  }
}

function bindClusterSelection(payload) {
  for (const node of document.querySelectorAll('[data-cluster-id]')) {
    node.addEventListener('click', async () => {
      try {
        await selectCluster(Number(node.getAttribute('data-cluster-id')));
      } catch (error) {
        console.error(error);
      }
    });
  }
}

function bindClusterSummaryCards() {
  for (const node of document.querySelectorAll('[data-summary-filter]')) {
    node.addEventListener('click', async () => {
      const clusterType = node.getAttribute('data-summary-filter') || '';
      clusterFilters = {
        clusterType,
        status: 'candidate',
      };
      const clusterTypeFilter = document.getElementById('clusterTypeFilter');
      const clusterStatusFilter = document.getElementById('clusterStatusFilter');
      if (clusterTypeFilter) {
        clusterTypeFilter.value = clusterType;
      }
      if (clusterStatusFilter) {
        clusterStatusFilter.value = 'candidate';
      }
      await loadClusters();
    });
  }
}

function bindFileSelection(payload, append) {
  for (const node of document.querySelectorAll('[data-file-id]')) {
    node.addEventListener('click', async () => {
      selectedFileID = Number(node.getAttribute('data-file-id'));
      const list = document.getElementById('filesList');
      if (list && !append) {
        list.innerHTML = payload.map(renderFile).join('');
        bindFileSelection(payload, false);
      } else {
        for (const item of document.querySelectorAll('[data-file-id]')) {
          item.setAttribute('data-selected', Number(item.getAttribute('data-file-id')) === selectedFileID ? 'true' : 'false');
        }
      }
      try {
        await loadFileDetail(selectedFileID);
      } catch (error) {
        console.error(error);
      }
    });
  }
  for (const checkbox of document.querySelectorAll('[data-file-select-id]')) {
    checkbox.addEventListener('click', (event) => {
      event.stopPropagation();
    });
    checkbox.addEventListener('change', () => {
      const fileID = Number(checkbox.getAttribute('data-file-select-id'));
      if (!fileID) {
        return;
      }
      if (checkbox.checked) {
        selectedFileIDs.add(fileID);
      } else {
        selectedFileIDs.delete(fileID);
      }
      const card = checkbox.closest('[data-file-id]');
      if (card) {
        card.setAttribute('data-bulk-selected', checkbox.checked ? 'true' : 'false');
      }
      renderBulkActionBar();
    });
  }
}

function bindVolumeActions() {
  for (const button of document.querySelectorAll('[data-scan-volume-id]')) {
    button.addEventListener('click', async () => {
      const volumeID = Number(button.getAttribute('data-scan-volume-id'));
      if (!volumeID) {
        return;
      }
      try {
        const response = await fetch(`/api/volumes/${volumeID}/scan`, { method: 'POST' });
        if (!response.ok) {
          throw new Error(`scan enqueue failed: ${response.status}`);
        }
        await refresh();
      } catch (error) {
        console.error(error);
      }
    });
  }
}

function bindTagSelection() {
  const tagInput = document.getElementById('fileTagFilter');
  const tagNamespaceSelect = document.getElementById('fileTagNamespaceFilter');
  for (const button of document.querySelectorAll('[data-tag-value]')) {
    button.addEventListener('click', async () => {
      const value = button.getAttribute('data-tag-value') || '';
      const namespace = button.getAttribute('data-tag-namespace') || '';
      fileFilters.tag = fileFilters.tag === value ? '' : value;
      fileFilters.tagNamespace = fileFilters.tag ? (namespace || fileFilters.tagNamespace) : '';
      if (tagInput) {
        tagInput.value = fileFilters.tag;
      }
      if (tagNamespaceSelect) {
        tagNamespaceSelect.value = fileFilters.tagNamespace;
      }
      fileOffset = 0;
      await loadFiles(false);
      await loadTags();
    });
  }
}

function bindFileDetailActions(fileID) {
  const revealButton = document.getElementById('revealFileButton');
  if (revealButton) {
    revealButton.addEventListener('click', async () => {
      try {
        const response = await fetch(`/api/files/${fileID}/reveal`, { method: 'POST' });
        if (!response.ok) {
          throw new Error(`reveal failed: ${response.status}`);
        }
      } catch (error) {
        console.error(error);
      }
    });
  }
  const keepButton = document.getElementById('keepFileButton');
  const recomputeEmbeddingsButton = document.getElementById('recomputeEmbeddingsButton');
  if (recomputeEmbeddingsButton) {
    recomputeEmbeddingsButton.addEventListener('click', async () => {
      try {
        const response = await fetch(`/api/files/${fileID}/recompute-embeddings`, { method: 'POST' });
        if (!response.ok) {
          throw new Error(`recompute embeddings failed: ${response.status}`);
        }
        await loadFileDetail(fileID);
        await loadJobs();
      } catch (error) {
        console.error(error);
      }
    });
  }
  const reclusterButton = document.getElementById('reclusterFileButton');
  if (reclusterButton) {
    reclusterButton.addEventListener('click', async () => {
      try {
        const response = await fetch(`/api/files/${fileID}/recluster`, { method: 'POST' });
        if (!response.ok) {
          throw new Error(`recluster failed: ${response.status}`);
        }
        await loadFileDetail(fileID);
        await loadClusters();
        await loadJobs();
      } catch (error) {
        console.error(error);
      }
    });
  }
  if (keepButton) {
    keepButton.addEventListener('click', async () => {
      try {
        await submitFileReviewAction(fileID, 'keep', 'manual keep');
        await refresh();
      } catch (error) {
        console.error(error);
      }
    });
  }
  const favoriteButton = document.getElementById('favoriteFileButton');
  if (favoriteButton) {
    favoriteButton.addEventListener('click', async () => {
      try {
        await submitFileReviewAction(fileID, 'favorite', 'manual favorite');
        await refresh();
      } catch (error) {
        console.error(error);
      }
    });
  }
  const trashButton = document.getElementById('trashFileButton');
  if (trashButton) {
    trashButton.addEventListener('click', async () => {
      const confirmed = window.confirm('Move this file to the macOS Trash?');
      if (!confirmed) {
        return;
      }
      try {
        const response = await fetch(`/api/files/${fileID}/trash`, { method: 'POST' });
        if (!response.ok) {
          throw new Error(`trash failed: ${response.status}`);
        }
        selectedFileID = null;
        await refresh();
      } catch (error) {
        console.error(error);
      }
    });
  }
  const tagForm = document.getElementById('fileTagForm');
  if (tagForm) {
    tagForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      const namespaceNode = document.getElementById('manualTagNamespace');
      const nameNode = document.getElementById('manualTagName');
      const displayNameNode = document.getElementById('manualTagDisplayName');
      if (!namespaceNode || !nameNode || !displayNameNode) {
        return;
      }
      const namespace = namespaceNode.value.trim();
      const name = nameNode.value.trim();
      const displayName = displayNameNode.value.trim();
      if (!namespace || !name) {
        return;
      }
      try {
        const response = await fetch(`/api/files/${fileID}/tags`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            namespace,
            name,
            display_name: displayName,
          }),
        });
        if (!response.ok) {
          throw new Error(`tag create failed: ${response.status}`);
        }
        nameNode.value = '';
        displayNameNode.value = '';
        await refresh();
      } catch (error) {
        console.error(error);
      }
    });
  }
  for (const button of document.querySelectorAll('[data-delete-tag-name]')) {
    button.addEventListener('click', async () => {
      const namespace = button.getAttribute('data-delete-tag-namespace') || '';
      const name = button.getAttribute('data-delete-tag-name') || '';
      if (!namespace || !name) {
        return;
      }
      try {
        const search = new URLSearchParams({ namespace, name });
        const response = await fetch(`/api/files/${fileID}/tags?${search.toString()}`, {
          method: 'DELETE',
        });
        if (!response.ok) {
          throw new Error(`tag delete failed: ${response.status}`);
        }
        await refresh();
      } catch (error) {
        console.error(error);
      }
    });
  }
}

function bindClusterMemberLinks() {
  for (const node of document.querySelectorAll('[data-open-file-id]')) {
    node.addEventListener('click', async () => {
      const fileID = Number(node.getAttribute('data-open-file-id'));
      if (!fileID) {
        return;
      }
      selectedFileID = fileID;
      try {
        await loadFileDetail(fileID);
      } catch (error) {
        console.error(error);
      }
    });
  }
}

function bindFileClusterLinks() {
  for (const node of document.querySelectorAll('[data-open-cluster-id]')) {
    node.addEventListener('click', async () => {
      const clusterID = Number(node.getAttribute('data-open-cluster-id'));
      const clusterType = node.getAttribute('data-open-cluster-type') || '';
      if (!clusterID) {
        return;
      }
      selectedClusterID = clusterID;
      clusterFilters = {
        clusterType,
        status: 'candidate',
      };
      const clusterTypeFilter = document.getElementById('clusterTypeFilter');
      const clusterStatusFilter = document.getElementById('clusterStatusFilter');
      if (clusterTypeFilter) {
        clusterTypeFilter.value = clusterType;
      }
      if (clusterStatusFilter) {
        clusterStatusFilter.value = 'candidate';
      }
      try {
        await loadClusters();
      } catch (error) {
        console.error(error);
      }
    });
  }
}

function bindClusterDetailActions(clusterID) {
  const previousButton = document.getElementById('previousClusterButton');
  if (previousButton) {
    previousButton.addEventListener('click', async () => {
      const previousCluster = getAdjacentCluster(clusterID, -1);
      if (!previousCluster) {
        return;
      }
      try {
        await selectCluster(previousCluster.id);
      } catch (error) {
        console.error(error);
      }
    });
  }
  const nextButton = document.getElementById('nextClusterButton');
  if (nextButton) {
    nextButton.addEventListener('click', async () => {
      const nextCluster = getAdjacentCluster(clusterID, 1);
      if (!nextCluster) {
        return;
      }
      try {
        await selectCluster(nextCluster.id);
      } catch (error) {
        console.error(error);
      }
    });
  }
  const confirmButton = document.getElementById('confirmClusterButton');
  if (confirmButton) {
    confirmButton.addEventListener('click', async () => {
      try {
        await submitClusterStatus(clusterID, 'confirmed');
        await advanceAfterClusterAction(clusterID, 'advance');
      } catch (error) {
        console.error(error);
      }
    });
  }
  const ignoreButton = document.getElementById('ignoreClusterButton');
  if (ignoreButton) {
    ignoreButton.addEventListener('click', async () => {
      try {
        await submitClusterStatus(clusterID, 'ignored');
        await advanceAfterClusterAction(clusterID, 'advance');
      } catch (error) {
        console.error(error);
      }
    });
  }
  const resetButton = document.getElementById('resetClusterButton');
  if (resetButton) {
    resetButton.addEventListener('click', async () => {
      try {
        await submitClusterStatus(clusterID, 'candidate');
        await advanceAfterClusterAction(clusterID, 'stay');
      } catch (error) {
        console.error(error);
      }
    });
  }
  const keepButton = document.getElementById('keepClusterButton');
  if (keepButton) {
    keepButton.addEventListener('click', async () => {
      try {
        await submitClusterReviewAction(clusterID, 'keep', 'manual cluster keep');
        await advanceAfterClusterAction(clusterID, 'advance');
      } catch (error) {
        console.error(error);
      }
    });
  }
  const favoriteButton = document.getElementById('favoriteClusterButton');
  if (favoriteButton) {
    favoriteButton.addEventListener('click', async () => {
      try {
        await submitClusterReviewAction(clusterID, 'favorite', 'manual cluster favorite');
        await advanceAfterClusterAction(clusterID, 'advance');
      } catch (error) {
        console.error(error);
      }
    });
  }
  const trashCandidateButton = document.getElementById('trashCandidateClusterButton');
  if (trashCandidateButton) {
    trashCandidateButton.addEventListener('click', async () => {
      try {
        await submitClusterReviewAction(clusterID, 'trash_candidate', 'manual cluster trash candidate');
        await advanceAfterClusterAction(clusterID, 'advance');
      } catch (error) {
        console.error(error);
      }
    });
  }
}

async function submitFileReviewAction(fileID, actionType, note) {
  const response = await fetch(`/api/files/${fileID}/review-actions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action_type: actionType, note }),
  });
  if (!response.ok) {
    throw new Error(`review action failed: ${response.status}`);
  }
}

function renderBulkActionBar() {
  const bar = document.getElementById('bulkActionBar');
  const countNode = document.getElementById('bulkSelectionCount');
  if (!bar || !countNode) {
    return;
  }
  const count = selectedFileIDs.size;
  bar.setAttribute('data-visible', count > 0 ? 'true' : 'false');
  countNode.textContent = `${count} selected`;
}

function renderActiveFilterBar() {
  const bar = document.getElementById('activeFilterBar');
  const chips = document.getElementById('activeFilterChips');
  if (!bar || !chips) {
    return;
  }
  const items = [];
  if (fileSearchQuery) {
    items.push(`Search: ${fileSearchQuery}`);
  }
  if (fileFilters.mediaType) {
    items.push(`Media: ${fileFilters.mediaType}`);
  }
  if (fileFilters.qualityTier) {
    items.push(`Quality: ${fileFilters.qualityTier}`);
  }
  if (fileFilters.reviewAction) {
    items.push(`Review: ${fileFilters.reviewAction}`);
  }
  if (fileFilters.status) {
    items.push(`Status: ${fileFilters.status}`);
  }
  if (fileFilters.volumeID) {
    items.push(`Volume: ${fileFilters.volumeID}`);
  }
  if (fileFilters.tagNamespace) {
    items.push(`Tag NS: ${fileFilters.tagNamespace}`);
  }
  if (fileFilters.tag) {
    items.push(`Tag: ${fileFilters.tag}`);
  }
  if (fileFilters.clusterType) {
    items.push(`Cluster: ${fileFilters.clusterType}`);
  }
  if (fileFilters.clusterStatus) {
    items.push(`Cluster Status: ${fileFilters.clusterStatus}`);
  }
  if (fileSort && fileSort !== 'updated_desc') {
    items.push(`Sort: ${fileSort}`);
  }
  bar.setAttribute('data-visible', items.length > 0 ? 'true' : 'false');
  chips.innerHTML = items.map((item) => `<span class="active-filter-chip">${escapeHTML(item)}</span>`).join('');
}

async function applyBulkFileReviewAction(actionType, note) {
  const ids = [...selectedFileIDs];
  if (ids.length === 0) {
    return;
  }
  for (const fileID of ids) {
    await submitFileReviewAction(fileID, actionType, note);
  }
  selectedFileIDs = new Set();
  await refresh();
}

async function submitClusterReviewAction(clusterID, actionType, note) {
  const response = await fetch(`/api/clusters/${clusterID}/review-actions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action_type: actionType, note }),
  });
  if (!response.ok) {
    throw new Error(`cluster review action failed: ${response.status}`);
  }
}

async function submitClusterStatus(clusterID, status) {
  const response = await fetch(`/api/clusters/${clusterID}/status`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  });
  if (!response.ok) {
    throw new Error(`cluster status update failed: ${response.status}`);
  }
}

function renderEvents(items) {
  const list = document.getElementById('jobEventsList');
  if (!list) {
    return;
  }
  if (!Array.isArray(items) || items.length === 0) {
    list.innerHTML = '<li class="event-row empty">No events yet.</li>';
    return;
  }
  list.innerHTML = items.map(renderEvent).join('');
}

function renderEvent(event) {
  return `
    <li class="event-row">
      <div class="event-meta">
        <span class="event-level">${escapeHTML(event.level || 'info')}</span>
        <span class="event-time">${escapeHTML(formatTimestamp(event.created_at))}</span>
      </div>
      <p class="event-message">${escapeHTML(event.message || '')}</p>
    </li>
  `;
}

function formatTimestamp(raw) {
  if (!raw) {
    return 'unknown time';
  }
  const value = new Date(raw);
  if (Number.isNaN(value.getTime())) {
    return String(raw);
  }
  return value.toLocaleString();
}

function formatDuration(durationMS) {
  const totalSeconds = Math.max(0, Math.floor(Number(durationMS) / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
}

function formatQuality(tier, score) {
  const parts = [];
  if (tier) {
    parts.push(String(tier));
  }
  if (typeof score === 'number') {
    parts.push(Number(score).toFixed(1));
  }
  return parts.length > 0 ? parts.join(' • ') : 'pending';
}

function formatBitrate(value) {
  const bitrate = Math.max(0, Number(value) || 0);
  if (bitrate >= 1_000_000) {
    return `${(bitrate / 1_000_000).toFixed(1)} Mbps`;
  }
  if (bitrate >= 1_000) {
    return `${Math.round(bitrate / 1_000)} Kbps`;
  }
  return `${Math.round(bitrate)} bps`;
}

function escapeHTML(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function summarizeSystemStatus(status, checks) {
  if (status === 'ready') {
    return 'All required dependencies are available.';
  }
  const failed = checks.filter((item) => item.status === 'not_ready');
  if (failed.length === 0) {
    return 'Runtime status is degraded.';
  }
  return failed.map((item) => `${item.name}: ${item.message || 'not ready'}`).join(' | ');
}

async function refresh() {
  try {
    await Promise.all([loadSystemStatus(), loadTaskSummary(), loadClusterSummary(), loadVolumes(), loadTags(), loadClusters(), loadFiles(), loadJobs()]);
  } catch (error) {
    console.error(error);
  }
}

document.addEventListener('DOMContentLoaded', () => {
  const refreshButton = document.getElementById('refreshButton');
  if (refreshButton) {
    refreshButton.addEventListener('click', refresh);
  }

  const volumeForm = document.getElementById('volumeForm');
  if (volumeForm) {
    volumeForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      const displayName = document.getElementById('volumeDisplayName');
      const mountPath = document.getElementById('volumeMountPath');
      if (!displayName || !mountPath) {
        return;
      }
      try {
        const response = await fetch('/api/volumes', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            display_name: displayName.value.trim(),
            mount_path: mountPath.value.trim(),
          }),
        });
        if (!response.ok) {
          throw new Error(`create volume failed: ${response.status}`);
        }
        displayName.value = '';
        mountPath.value = '';
        await refresh();
      } catch (error) {
        console.error(error);
      }
    });
  }

  const clusterFilterForm = document.getElementById('clusterFilterForm');
  if (clusterFilterForm) {
    clusterFilterForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      const clusterType = document.getElementById('clusterTypeFilter');
      const status = document.getElementById('clusterStatusFilter');
      if (!clusterType || !status) {
        return;
      }
      clusterFilters = {
        clusterType: clusterType.value,
        status: status.value,
      };
      await loadClusters();
    });
  }

  const fileSearchForm = document.getElementById('fileSearchForm');
  if (fileSearchForm) {
    fileSearchForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      const input = document.getElementById('fileSearchInput');
      const mediaType = document.getElementById('fileMediaTypeFilter');
      const qualityTier = document.getElementById('fileQualityTierFilter');
      const reviewAction = document.getElementById('fileReviewActionFilter');
      const status = document.getElementById('fileStatusFilter');
      const volumeID = document.getElementById('fileVolumeFilter');
      const tagNamespaceNode = document.getElementById('fileTagNamespaceFilter');
      const tag = document.getElementById('fileTagFilter');
      const clusterType = document.getElementById('fileClusterTypeFilter');
      const clusterStatus = document.getElementById('fileClusterStatusFilter');
      const sort = document.getElementById('fileSortFilter');
      if (!input || !mediaType || !qualityTier || !reviewAction || !status || !volumeID || !tagNamespaceNode || !tag || !clusterType || !clusterStatus || !sort) {
        return;
      }
      fileSearchQuery = input.value.trim();
      fileFilters = {
        mediaType: mediaType.value,
        qualityTier: qualityTier.value,
        reviewAction: reviewAction.value,
        status: status.value,
        volumeID: volumeID.value,
        tagNamespace: tagNamespaceNode.value,
        tag: tag.value.trim(),
        clusterType: clusterType.value,
        clusterStatus: clusterStatus.value,
      };
      fileSort = sort.value || 'updated_desc';
      fileOffset = 0;
      await loadFiles(false);
      renderActiveFilterBar();
    });
  }

  const filesLoadMoreButton = document.getElementById('filesLoadMoreButton');
  if (filesLoadMoreButton) {
    filesLoadMoreButton.addEventListener('click', async () => {
      if (!hasMoreFiles) {
        return;
      }
      fileOffset += FILES_PAGE_SIZE;
      await loadFiles(true);
    });
  }

  const selectVisibleFilesButton = document.getElementById('selectVisibleFilesButton');
  if (selectVisibleFilesButton) {
    selectVisibleFilesButton.addEventListener('click', () => {
      for (const node of document.querySelectorAll('[data-file-id]')) {
        const fileID = Number(node.getAttribute('data-file-id'));
        if (fileID) {
          selectedFileIDs.add(fileID);
        }
      }
      for (const checkbox of document.querySelectorAll('[data-file-select-id]')) {
        checkbox.checked = true;
      }
      renderBulkActionBar();
    });
  }

  const clearSelectedFilesButton = document.getElementById('clearSelectedFilesButton');
  if (clearSelectedFilesButton) {
    clearSelectedFilesButton.addEventListener('click', () => {
      selectedFileIDs = new Set();
      for (const checkbox of document.querySelectorAll('[data-file-select-id]')) {
        checkbox.checked = false;
      }
      for (const item of document.querySelectorAll('[data-file-id]')) {
        item.setAttribute('data-bulk-selected', 'false');
      }
      renderBulkActionBar();
    });
  }

  const bulkKeepFilesButton = document.getElementById('bulkKeepFilesButton');
  if (bulkKeepFilesButton) {
    bulkKeepFilesButton.addEventListener('click', async () => {
      try {
        await applyBulkFileReviewAction('keep', 'bulk keep');
      } catch (error) {
        console.error(error);
      }
    });
  }

  const bulkFavoriteFilesButton = document.getElementById('bulkFavoriteFilesButton');
  if (bulkFavoriteFilesButton) {
    bulkFavoriteFilesButton.addEventListener('click', async () => {
      try {
        await applyBulkFileReviewAction('favorite', 'bulk favorite');
      } catch (error) {
        console.error(error);
      }
    });
  }

  const bulkTrashCandidateFilesButton = document.getElementById('bulkTrashCandidateFilesButton');
  if (bulkTrashCandidateFilesButton) {
    bulkTrashCandidateFilesButton.addEventListener('click', async () => {
      try {
        await applyBulkFileReviewAction('trash_candidate', 'bulk trash candidate');
      } catch (error) {
        console.error(error);
      }
    });
  }

  const tagNamespaceFilter = document.getElementById('tagNamespaceFilter');
  if (tagNamespaceFilter) {
    tagNamespaceFilter.addEventListener('change', async () => {
      tagNamespace = tagNamespaceFilter.value;
      await loadTags();
    });
  }

  refreshTimer = window.setInterval(refresh, 5000);
  bindClusterSummaryCards();
  bindKeyboardShortcuts();
  renderBulkActionBar();
  renderActiveFilterBar();
  void refresh();
});

window.addEventListener('beforeunload', () => {
  if (refreshTimer !== null) {
    window.clearInterval(refreshTimer);
  }
});
