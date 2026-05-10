<script setup>
import { ref, computed } from 'vue';
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

const loading = ref(false);

const title = computed(() =>
  props.mode === 'edit' ? t('amnezia.editServer') : t('amnezia.addServer')
);

function resetForm() {
  if (props.mode === 'edit' && props.server) {
    form.value = { ...props.server };
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
  }
}

async function onSubmit() {
  loading.value = true;
  try {
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
</script>

<template>
  <a-modal
    :open="props.open"
    :title="title"
    :confirm-loading="loading"
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

      <a-form-item :label="t('amnezia.listenPort')" required>
        <a-input-number v-model:value="form.listenPort" :min="1" :max="65535" style="width: 100%" />
      </a-form-item>

      <a-form-item :label="t('amnezia.address')" required>
        <a-input v-model:value="form.address" placeholder="10.0.0.1/24" />
      </a-form-item>

      <a-form-item :label="t('amnezia.dns')">
        <a-input v-model:value="form.dns" placeholder="8.8.4.4" />
      </a-form-item>

      <a-form-item :label="t('amnezia.mtu')">
        <a-input-number v-model:value="form.mtu" :min="576" :max="1500" style="width: 100%" />
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

      <a-form-item :label="t('amnezia.serverEnabled')">
        <a-switch v-model:checked="form.enabled" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>