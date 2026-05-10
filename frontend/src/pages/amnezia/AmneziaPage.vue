<script setup>
import { computed, h, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import axios from 'axios';
import {
  CloudServerOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  DownloadOutlined,
  QrcodeOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  ReloadOutlined,
  ClockCircleOutlined,
  LinkOutlined,
} from '@ant-design/icons-vue';

import { FileManager, HttpUtil, SizeFormatter } from '@/utils';
import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import { useWebSocket } from '@/composables/useWebSocket.js';
import AppSidebar from '@/components/AppSidebar.vue';
import CustomStatistic from '@/components/CustomStatistic.vue';
import ServerFormModal from './ServerFormModal.vue';
import PeerFormModal from './PeerFormModal.vue';
import ExtendPeerModal from './ExtendPeerModal.vue';

const { t } = useI18n();
const { isMobile } = useMediaQuery();

const basePath = window.__X_UI_BASE_PATH__ || '';
const requestUri = window.location.pathname;

const servers = ref([]);
const peers = ref([]);
const runtimeRows = ref([]);
const selectedServerId = ref(null);
const loading = ref(false);
const fetched = ref(false);

const serverFormOpen = ref(false);
const serverFormMode = ref('add');
const serverFormServer = ref(null);

const peerFormOpen = ref(false);
const peerFormMode = ref('add');
const peerFormPeer = ref(null);
const peerFormServerId = ref(null);

const extendPeerOpen = ref(false);
const extendPeerId = ref(null);

onMounted(async () => {
  await fetchServers();
});

useWebSocket({
  amnezia: applyRuntimeSnapshot,
});

async function fetchServers() {
  loading.value = true;
  try {
    const msg = await HttpUtil.get('/panel/api/amnezia/runtime');
    if (msg?.success) {
      applyRuntimeSnapshot(msg.obj);
    }
  } finally {
    loading.value = false;
    fetched.value = true;
  }
}

async function fetchPeers(serverId) {
  selectedServerId.value = serverId;
  syncPeersFromRuntime();
}

function applyRuntimeSnapshot(snapshot) {
  const rows = Array.isArray(snapshot?.servers) ? snapshot.servers : [];
  runtimeRows.value = rows;
  servers.value = rows.map(row => ({
    ...row.server,
    running: row.running,
    up: row.up || 0,
    down: row.down || 0,
    online: row.online || 0,
    peerCount: Array.isArray(row.peers) ? row.peers.length : 0,
  }));
  if (!selectedServerId.value && servers.value.length) {
    selectedServerId.value = servers.value[0].id;
  }
  if (selectedServerId.value && !servers.value.some(server => server.id === selectedServerId.value)) {
    selectedServerId.value = servers.value[0]?.id || null;
  }
  syncPeersFromRuntime();
}

function syncPeersFromRuntime() {
  const row = runtimeRows.value.find(item => item.server?.id === selectedServerId.value);
  peers.value = (row?.peers || []).map(item => ({
    ...item.peer,
    stat: item.stat || {},
    online: !!item.online,
    usage: item.usage || 0,
    expired: !!item.expired,
    trafficLimited: !!item.trafficLimited,
  }));
}

function onAddServer() {
  serverFormMode.value = 'add';
  serverFormServer.value = null;
  serverFormOpen.value = true;
}

function onEditServer(server) {
  serverFormMode.value = 'edit';
  serverFormServer.value = { ...server };
  serverFormOpen.value = true;
}

async function onDeleteServer(server) {
  const msg = await HttpUtil.delete(`/panel/api/amnezia/servers/${server.id}`);
  if (msg?.success) {
    message.success(t('amnezia.toasts.serverDeleted'));
    await fetchServers();
  }
}

async function onSaveServer(payload) {
  const url = serverFormMode.value === 'edit'
    ? `/panel/api/amnezia/servers/${serverFormServer.value.id}`
    : '/panel/api/amnezia/servers';
  const method = serverFormMode.value === 'edit' ? HttpUtil.put : HttpUtil.post;
  const msg = await method(url, payload);
  if (msg?.success) {
    message.success(serverFormMode.value === 'edit'
      ? t('amnezia.toasts.serverUpdated')
      : t('amnezia.toasts.serverCreated'));
    serverFormOpen.value = false;
    await fetchServers();
    return true;
  }
  return false;
}

async function onStartServer(server) {
  const msg = await HttpUtil.post(`/panel/api/amnezia/servers/${server.id}/start`);
  if (msg?.success) {
    message.success(t('amnezia.toasts.serverStarted'));
    await fetchServers();
  }
}

async function onStopServer(server) {
  const msg = await HttpUtil.post(`/panel/api/amnezia/servers/${server.id}/stop`);
  if (msg?.success) {
    message.success(t('amnezia.toasts.serverStopped'));
    await fetchServers();
  }
}

async function onRestartServer(server) {
  const msg = await HttpUtil.post(`/panel/api/amnezia/servers/${server.id}/restart`);
  if (msg?.success) {
    message.success(t('amnezia.toasts.serverRestarted'));
    await fetchServers();
  }
}

function onAddPeer(serverId) {
  if (!serverId) return;
  peerFormMode.value = 'add';
  peerFormPeer.value = null;
  peerFormServerId.value = serverId;
  peerFormOpen.value = true;
}

function onEditPeer(peer) {
  peerFormMode.value = 'edit';
  peerFormPeer.value = { ...peer };
  peerFormServerId.value = peer.serverId;
  peerFormOpen.value = true;
}

async function onDeletePeer(peer) {
  const msg = await HttpUtil.delete(`/panel/api/amnezia/peers/${peer.id}`);
  if (msg?.success) {
    message.success(t('amnezia.toasts.peerDeleted'));
    await fetchServers();
  }
}

async function onSavePeer(payload) {
  const url = peerFormMode.value === 'edit'
    ? `/panel/api/amnezia/peers/${peerFormPeer.value.id}`
    : `/panel/api/amnezia/servers/${peerFormServerId.value}/peers`;
  const method = peerFormMode.value === 'edit' ? HttpUtil.put : HttpUtil.post;
  const msg = await method(url, payload);
  if (msg?.success) {
    message.success(peerFormMode.value === 'edit'
      ? t('amnezia.toasts.peerUpdated')
      : t('amnezia.toasts.peerCreated'));
    peerFormOpen.value = false;
    await fetchServers();
    return true;
  }
  return false;
}

function onExtendPeer(peer) {
  extendPeerId.value = peer.id;
  extendPeerOpen.value = true;
}

async function onExtendPeerConfirm(days) {
  const msg = await HttpUtil.post(`/panel/api/amnezia/peers/${extendPeerId.value}/extend`, { days });
  if (msg?.success) {
    message.success(t('amnezia.toasts.peerExtended'));
    extendPeerOpen.value = false;
    await fetchServers();
    return true;
  }
  return false;
}

async function onDownloadConfig(peer) {
  try {
    const resp = await axios.get(`/panel/api/amnezia/peers/${peer.id}/config`, { responseType: 'text' });
    FileManager.downloadTextFile(resp.data || '', `${peer.name || 'peer'}.conf`);
  } catch (err) {
    message.error(err?.response?.data?.msg || err.message || t('amnezia.toasts.copyFailed'));
  }
}

async function onShowQRCode(peer) {
  const msg = await HttpUtil.get(`/panel/api/amnezia/peers/${peer.id}/qrcode`);
  if (msg?.success && msg.obj?.qr) {
    Modal.info({
      title: t('amnezia.showQRCode'),
      content: () => h('img', { src: msg.obj.qr, style: { maxWidth: '100%', display: 'block', margin: '0 auto' } }),
      okText: t('close'),
      width: 400,
      onOk: () => {},
    });
  }
}

async function onShowVpnUri(peer) {
  const msg = await HttpUtil.get(`/panel/api/amnezia/peers/${peer.id}/vpnuri`);
  if (msg?.success && msg.obj?.vpnUri) {
    Modal.info({
      title: t('amnezia.vpnUri'),
      content: () => h('div', {
        style: { wordBreak: 'break-all', fontFamily: 'monospace', fontSize: '12px', padding: '8px', background: '#f5f5f5', borderRadius: '4px' }
      }, msg.obj.vpnUri),
      okText: t('close'),
      width: 520,
      onOk: () => {},
    });
  }
}

async function onCopyVpnUri(peer) {
  const msg = await HttpUtil.get(`/panel/api/amnezia/peers/${peer.id}/vpnuri`);
  if (msg?.success && msg.obj?.vpnUri) {
    try {
      await navigator.clipboard.writeText(msg.obj.vpnUri);
      message.success(t('amnezia.toasts.copied'));
    } catch (err) {
      message.error(t('amnezia.toasts.copyFailed'));
    }
  }
}

function getPeerStatus(peer) {
  if (peer.trafficLimited) {
    return { label: 'Traffic limit', severity: 'red' };
  }
  if (!peer.enabled && peer.pausedReason === 'expired') {
    return { label: t('amnezia.statusExpired'), severity: 'danger' };
  }
  if (!peer.enabled) {
    return { label: t('amnezia.statusPaused'), severity: 'warning' };
  }
  if (!peer.expiresAt) {
    return { label: t('amnezia.statusActiveUnlimited'), severity: 'success' };
  }
  const expiresAt = new Date(peer.expiresAt * 1000);
  const now = new Date();
  if (expiresAt <= now) {
    return { label: t('amnezia.statusExpired'), severity: 'danger' };
  }
  return { label: t('amnezia.statusActive'), severity: 'success' };
}

function onSelectServer(server) {
  fetchPeers(server.id);
}

function formatTraffic(bytes) {
  return SizeFormatter.sizeFormat(Number(bytes || 0));
}

function formatHandshake(ts) {
  if (!ts) return '-';
  return new Date(ts * 1000).toLocaleString();
}

function trafficLimitLabel(peer) {
  if (!peer.trafficLimit) return 'Unlimited';
  return `${formatTraffic(peer.usage)} / ${formatTraffic(peer.trafficLimit)}`;
}

function getDaysLeft(expiresAt) {
  if (!expiresAt) return 'Unlimited';
  const now = new Date();
  const end = new Date(expiresAt * 1000);
  const diffMs = end.getTime() - now.getTime();
  if (diffMs <= 0) return '0';
  return Math.ceil(diffMs / (1000 * 60 * 60 * 24)).toString();
}

function formatDate(ts) {
  if (!ts) return '-';
  return new Date(ts * 1000).toLocaleDateString();
}

const totals = computed(() => {
  const total = servers.value.length;
  const enabled = servers.value.filter(s => s.running).length;
  const disabled = total - enabled;
  const online = servers.value.reduce((sum, server) => sum + (server.online || 0), 0);
  const usage = servers.value.reduce((sum, server) => sum + (server.up || 0) + (server.down || 0), 0);
  return { total, enabled, disabled, online, usage };
});

const selectedServer = computed(() => servers.value.find(server => server.id === selectedServerId.value) || null);
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="amnezia-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content class="content-area">
          <a-spin :spinning="loading || !fetched" :delay="200" tip="Loading..." size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-row v-else :gutter="[isMobile ? 8 : 16, isMobile ? 0 : 12]">
              <a-col :span="24">
                <a-card size="small" hoverable class="summary-card">
                  <a-row :gutter="[16, 12]">
                    <a-col :sm="8" :md="6">
                      <CustomStatistic :title="t('amnezia.servers')" :value="String(totals.total)">
                        <template #prefix>
                          <CloudServerOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="8" :md="6">
                      <CustomStatistic :title="t('amnezia.statusActive')" :value="String(totals.enabled)">
                        <template #prefix>
                          <CheckCircleOutlined style="color: #52c41a" />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="8" :md="6">
                      <CustomStatistic :title="t('amnezia.statusPaused')" :value="String(totals.disabled)">
                        <template #prefix>
                          <CloseCircleOutlined style="color: #ff4d4f" />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="8" :md="6">
                      <CustomStatistic title="Online peers" :value="String(totals.online)">
                        <template #prefix>
                          <LinkOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="8" :md="6">
                      <CustomStatistic :title="t('amnezia.traffic')" :value="formatTraffic(totals.usage)">
                        <template #prefix>
                          <DownloadOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                  </a-row>
                </a-card>
              </a-col>

              <a-col :span="24">
                <a-card :title="t('amnezia.servers')" hoverable>
                  <template #extra>
                    <a-button type="primary" @click="onAddServer">
                      <PlusOutlined />
                      {{ t('amnezia.addServer') }}
                    </a-button>
                  </template>

                  <a-table
                    :data-source="servers"
                    :row-key="record => record.id"
                    :pagination="{ pageSize: 10 }"
                    :row-class-name="record => record.id === selectedServerId ? 'selected-row' : ''"
                    :custom-row="record => ({ onClick: () => onSelectServer(record) })"
                  >
                    <a-table-column :title="t('amnezia.serverName')" data-index="name" />
                    <a-table-column :title="t('amnezia.interfaceName')" data-index="interfaceName" />
                    <a-table-column :title="t('amnezia.listenPort')" data-index="listenPort" />
                    <a-table-column :title="t('amnezia.protocolMode')" data-index="protocolMode" />
                    <a-table-column title="Peers" data-index="peerCount" />
                    <a-table-column :title="t('amnezia.traffic')">
                      <template #default="{ record }">
                        <a-tag>{{ formatTraffic((record.up || 0) + (record.down || 0)) }}</a-tag>
                      </template>
                    </a-table-column>
                    <a-table-column :title="t('amnezia.status')" data-index="enabled">
                      <template #default="{ record }">
                        <a-tag :color="record.running ? 'green' : 'red'">
                          {{ record.running ? t('amnezia.statusActive') : t('amnezia.statusPaused') }}
                        </a-tag>
                      </template>
                    </a-table-column>
                    <a-table-column :title="t('actions')" :width="200">
                      <template #default="{ record }">
                        <a-space>
                          <a-tooltip :title="t('amnezia.editServer')">
                            <a-button size="small" @click.stop="onEditServer(record)">
                              <EditOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('amnezia.startServer')">
                            <a-button size="small" @click.stop="onStartServer(record)" :disabled="record.running">
                              <PlayCircleOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('amnezia.stopServer')">
                            <a-button size="small" @click.stop="onStopServer(record)" :disabled="!record.running">
                              <PauseCircleOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('amnezia.restartServer')">
                            <a-button size="small" @click.stop="onRestartServer(record)">
                              <ReloadOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-popconfirm :title="`${t('amnezia.deleteServer')}: ${record.name}`" @confirm="onDeleteServer(record)">
                            <a-button size="small" danger @click.stop>
                              <DeleteOutlined />
                            </a-button>
                          </a-popconfirm>
                        </a-space>
                      </template>
                    </a-table-column>
                  </a-table>
                </a-card>
              </a-col>

              <a-col :span="24">
                <a-card :title="selectedServer ? `${t('amnezia.peers')}: ${selectedServer.name}` : t('amnezia.peers')" hoverable>
                  <template #extra>
                    <a-button type="primary" @click="onAddPeer(selectedServerId)" :disabled="!selectedServerId">
                      <PlusOutlined />
                      {{ t('amnezia.addPeer') }}
                    </a-button>
                  </template>

                  <a-table :data-source="peers" :row-key="record => record.id" :pagination="{ pageSize: 10 }">
                    <a-table-column :title="t('amnezia.peerName')" data-index="name" />
                    <a-table-column :title="t('amnezia.address')" data-index="address" />
                    <a-table-column :title="t('amnezia.allowedIPs')" data-index="allowedIps" />
                    <a-table-column :title="t('amnezia.traffic')">
                      <template #default="{ record }">
                        <a-tag :color="record.trafficLimited ? 'red' : 'blue'">
                          {{ trafficLimitLabel(record) }}
                        </a-tag>
                      </template>
                    </a-table-column>
                    <a-table-column title="Last handshake">
                      <template #default="{ record }">
                        {{ formatHandshake(record.stat?.lastHandshake) }}
                      </template>
                    </a-table-column>
                    <a-table-column :title="t('amnezia.expiryDays')" data-index="expiryDays">
                      <template #default="{ record }">
                        {{ record.expiryDays || 'Unlimited' }}
                      </template>
                    </a-table-column>
                    <a-table-column :title="t('amnezia.expiresAt')" data-index="expiresAt">
                      <template #default="{ record }">
                        {{ formatDate(record.expiresAt) }}
                      </template>
                    </a-table-column>
                    <a-table-column :title="t('amnezia.daysLeft')" data-index="expiresAt">
                      <template #default="{ record }">
                        {{ getDaysLeft(record.expiresAt) }}
                      </template>
                    </a-table-column>
                    <a-table-column :title="t('amnezia.status')" data-index="enabled">
                      <template #default="{ record }">
                        <a-tag :color="getPeerStatus(record).severity">
                          {{ getPeerStatus(record).label }}
                        </a-tag>
                        <a-tag v-if="record.online" color="green">Online</a-tag>
                      </template>
                    </a-table-column>
                    <a-table-column :title="t('actions')" :width="200">
                      <template #default="{ record }">
                        <a-space>
                          <a-tooltip :title="t('amnezia.editPeer')">
                            <a-button size="small" @click="onEditPeer(record)">
                              <EditOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('amnezia.extendClient')">
                            <a-button size="small" @click="onExtendPeer(record)">
                              <ClockCircleOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('amnezia.downloadConfig')">
                            <a-button size="small" @click="onDownloadConfig(record)">
                              <DownloadOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('amnezia.showQRCode')">
                            <a-button size="small" @click="onShowQRCode(record)">
                              <QrcodeOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('amnezia.copyVpnUri')">
                            <a-button size="small" @click="onCopyVpnUri(record)">
                              <LinkOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-popconfirm :title="`${t('amnezia.deletePeer')}: ${record.name}`" @confirm="onDeletePeer(record)">
                            <a-button size="small" danger @click.stop>
                              <DeleteOutlined />
                            </a-button>
                          </a-popconfirm>
                        </a-space>
                      </template>
                    </a-table-column>
                  </a-table>
                </a-card>
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <ServerFormModal
        v-model:open="serverFormOpen"
        :mode="serverFormMode"
        :server="serverFormServer"
        :save="onSaveServer"
      />

      <PeerFormModal
        v-model:open="peerFormOpen"
        :mode="peerFormMode"
        :peer="peerFormPeer"
        :server-id="peerFormServerId"
        :save="onSavePeer"
      />

      <ExtendPeerModal
        v-model:open="extendPeerOpen"
        :peer-id="extendPeerId"
        :extend="onExtendPeerConfirm"
      />
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.amnezia-page {
  --bg-page: #e6e8ec;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.amnezia-page.is-dark {
  --bg-page: #0a1222;
  --bg-card: #151f31;
}

.amnezia-page.is-dark.is-ultra {
  --bg-page: #050505;
  --bg-card: #0c0e12;
}

.amnezia-page :deep(.ant-layout),
.amnezia-page :deep(.ant-layout-content) {
  background: transparent;
}

.content-shell {
  background: transparent;
}

.content-area {
  padding: 16px;
}

.loading-spacer {
  min-height: 200px;
}

.summary-card {
  margin-bottom: 16px;
}

.amnezia-page :deep(.selected-row td) {
  background: rgba(22, 119, 255, 0.08);
}

.amnezia-page :deep(.ant-table-row) {
  cursor: pointer;
}
</style>
