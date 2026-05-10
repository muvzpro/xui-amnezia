<script setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { message } from 'ant-design-vue';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add' },
  server: { type: Object, default: null },
  save: { type: Function, required: true },
});

const emit = defineEmits(['update:open']);

const form = ref({
  name: '',
  interfaceName: '',
  listenPort: 51820,
  address: '10.0.0.1/24',
  dns: '8.8.4.4',
  mtu: 1280,
  endpoint: '',
  protocolMode: 'AmneziaWG',
  obfuscationJson: '{}',
  enabled: true,
});

// AmneziaWG 2.0 obfuscation parameters
const obfuscation = ref({
  Jc: 5,
  Jmin: 50,
  Jmax: 200,
  S1: 72,
  S2: 56,
  S3: 32,
  S4: 16,
  H1: '100000-200000',
  H2: '300000-400000',
  H3: '500000-600000',
  H4: '700000-800000',
});

const showObfuscation = ref(false);

const loading = ref(false);

const title = computed(() =>
  props.mode === 'edit' ? t('amnezia.editServer') : t('amnezia.addServer')
);

// Parse obfuscation JSON when server changes
watch(() => props.server, (newServer) => {
  if (newServer?.obfuscationJson) {
    try {
      const parsed = JSON.parse(newServer.obfuscationJson);
      obfuscation.value = { ...obfuscation.value, ...parsed };
    } catch (e) {
      console.error('Failed to parse obfuscation JSON:', e);
    }
  }
}, { immediate: true });

function resetForm() {
  if (props.mode === 'edit' && props.server) {
    form.value = { ...props.server };
    if (props.server.obfuscationJson) {
      try {
        const parsed = JSON.parse(props.server.obfuscationJson);
        obfuscation.value = { ...obfuscation.value, ...parsed };
      } catch (e) {
        console.error('Failed to parse obfuscation JSON:', e);
      }
    }
  } else {
    form.value = {
      name: '',
      interfaceName: '',
      listenPort: 51820,
      address: '10.0.0.1/24',
      dns: '8.8.4.4',
      mtu: 1280,
      endpoint: '',
      protocolMode: 'AmneziaWG',
      obfuscationJson: '{}',
      enabled: true,
    };
    obfuscation.value = {
      Jc: 5,
      Jmin: 50,
      Jmax: 200,
      S1: 72,
      S2: 56,
      S3: 32,
      S4: 16,
      H1: '100000-200000',
      H2: '300000-400000',
      H3: '500000-600000',
      H4: '700000-800000',
    };
  }
}

async function onSubmit() {
  loading.value = true;
  try {
    // Include obfuscation parameters in JSON
    form.value.obfuscationJson = JSON.stringify(obfuscation.value);
    const ok = await props.save(form.value);
    if (ok) {
      emit('update:open', false);
    }
  } finally {
    loading.value = false;
  }
}

function onClose() {
  emit('update:open', false);
}

function generateRandomObfuscation() {
  obfuscation.value = {
    Jc: Math.floor(Math.random() * 9) + 4, // 4-12
    Jmin: Math.floor(Math.random() * 64) + 64, // 64-128
    Jmax: Math.floor(Math.random() * 256) + 768, // 768-1024
    S1: Math.floor(Math.random() * 65), // 0-64
    S2: Math.floor(Math.random() * 65), // 0-64
    S3: Math.floor(Math.random() * 65), // 0-64
    S4: Math.floor(Math.random() * 33), // 0-32
    H1: generateHeaderRange(),
    H2: generateHeaderRange(),
    H3: generateHeaderRange(),
    H4: generateHeaderRange(),
  };
}

function generateHeaderRange() {
  const start = Math.floor(Math.random() * 1000000);
  const end = start + Math.floor(Math.random() * 100000) + 100000;
  return `${start}-${end}`;
}
</script>

<template>
  <a-modal
    :open="props.open"
    :title="title"
    :confirm-loading="loading"
    :width="720"
    @ok="onSubmit"
    @cancel="onClose"
    @after-open-change="(visible) => visible && resetForm()"
  >
    <a-form layout="vertical">
      <a-form-item :label="t('amnezia.serverName')" required>
        <a-input v-model:value="form.name" :placeholder="t('amnezia.serverName')" />
      </a-form-item>

      <a-form-item :label="t('amnezia.interfaceName')" required>
        <a-input v-model:value="form.interfaceName" placeholder="awg0" />
      </a-form-item>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item :label="t('amnezia.listenPort')" required>
            <a-input-number v-model:value="form.listenPort" :min="1" :max="65535" style="width: 100%" />
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('amnezia.mtu')">
            <a-input-number v-model:value="form.mtu" :min="576" :max="1500" style="width: 100%" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item :label="t('amnezia.address')" required>
        <a-input v-model:value="form.address" placeholder="10.0.0.1/24" />
      </a-form-item>

      <a-form-item :label="t('amnezia.dns')">
        <a-input v-model:value="form.dns" placeholder="8.8.4.4" />
      </a-form-item>

      <a-form-item :label="t('amnezia.endpoint')">
        <a-input v-model:value="form.endpoint" placeholder="example.com:51820" />
      </a-form-item>

      <a-form-item :label="t('amnezia.protocolMode')">
        <a-select v-model:value="form.protocolMode">
          <a-select-option value="AmneziaWG">{{ t('amnezia.amneziaWG') }}</a-select-option>
          <a-select-option value="AmneziaWG20">{{ t('amnezia.amneziaWG20') }}</a-select-option>
        </a-select>
      </a-form-item>

      <!-- AmneziaWG 2.0 Obfuscation Parameters -->
      <a-divider>
        <a-button type="link" @click="showObfuscation = !showObfuscation">
          {{ t('amnezia.obfuscationParams') }}
          {{ showObfuscation ? '▲' : '▼' }}
        </a-button>
      </a-divider>

      <div v-if="showObfuscation">
        <a-space direction="vertical" style="width: 100%">
          <a-button size="small" @click="generateRandomObfuscation">
            {{ t('amnezia.generateRandom') }}
          </a-button>

          <a-row :gutter="16">
            <a-col :span="8">
              <a-form-item label="Jc">
                <a-input-number v-model:value="obfuscation.Jc" :min="1" :max="128" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item label="Jmin">
                <a-input-number v-model:value="obfuscation.Jmin" :min="0" :max="1280" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item label="Jmax">
                <a-input-number v-model:value="obfuscation.Jmax" :min="0" :max="1280" style="width: 100%" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-row :gutter="16">
            <a-col :span="6">
              <a-form-item label="S1">
                <a-input-number v-model:value="obfuscation.S1" :min="0" :max="64" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :span="6">
              <a-form-item label="S2">
                <a-input-number v-model:value="obfuscation.S2" :min="0" :max="64" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :span="6">
              <a-form-item label="S3">
                <a-input-number v-model:value="obfuscation.S3" :min="0" :max="64" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :span="6">
              <a-form-item label="S4">
                <a-input-number v-model:value="obfuscation.S4" :min="0" :max="32" style="width: 100%" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item label="H1">
                <a-input v-model:value="obfuscation.H1" placeholder="100000-200000" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item label="H2">
                <a-input v-model:value="obfuscation.H2" placeholder="300000-400000" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item label="H3">
                <a-input v-model:value="obfuscation.H3" placeholder="500000-600000" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item label="H4">
                <a-input v-model:value="obfuscation.H4" placeholder="700000-800000" />
              </a-form-item>
            </a-col>
          </a-row>
        </a-space>
      </div>

      <a-form-item :label="t('amnezia.serverEnabled')">
        <a-switch v-model:checked="form.enabled" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>