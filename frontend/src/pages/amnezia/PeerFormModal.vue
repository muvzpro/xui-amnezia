<script setup>
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add' },
  peer: { type: Object, default: null },
  serverId: { type: Number, default: null },
  save: { type: Function, required: true },
});

const emit = defineEmits(['update:open']);

const form = ref({
  name: '',
  address: '',
  allowedIps: '0.0.0.0/0, ::/0',
  persistentKeepalive: 25,
  expiryDays: null,
  enabled: true,
});

const loading = ref(false);

const title = computed(() =>
  props.mode === 'edit' ? t('amnezia.editPeer') : t('amnezia.addPeer')
);

function resetForm() {
  if (props.mode === 'edit' && props.peer) {
    form.value = {
      name: props.peer.name || '',
      address: props.peer.address || '',
      allowedIps: props.peer.allowedIps || '0.0.0.0/0, ::/0',
      persistentKeepalive: props.peer.persistentKeepalive || 25,
      expiryDays: props.peer.expiryDays || null,
      enabled: props.peer.enabled ?? true,
    };
  } else {
    form.value = {
      name: '',
      address: '',
      allowedIps: '0.0.0.0/0, ::/0',
      persistentKeepalive: 25,
      expiryDays: null,
      enabled: true,
    };
  }
}

async function onSubmit() {
  loading.value = true;
  try {
    const payload = {
      ...form.value,
      serverId: props.serverId,
    };
    const ok = await props.save(payload);
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
      <a-form-item :label="t('amnezia.peerName')" required>
        <a-input v-model:value="form.name" :placeholder="t('amnezia.peerName')" />
      </a-form-item>

      <a-form-item :label="t('amnezia.address')">
        <a-input v-model:value="form.address" placeholder="10.0.0.2/32" />
      </a-form-item>

      <a-form-item :label="t('amnezia.allowedIPs')">
        <a-input v-model:value="form.allowedIps" placeholder="0.0.0.0/0, ::/0" />
      </a-form-item>

      <a-form-item :label="t('amnezia.persistentKeepalive')">
        <a-input-number v-model:value="form.persistentKeepalive" :min="0" :max="65535" style="width: 100%" />
      </a-form-item>

      <a-form-item :label="t('amnezia.expiryDays')">
        <a-input-number v-model:value="form.expiryDays" :min="0" :placeholder="t('amnezia.expiryDaysPlaceholder')" style="width: 100%" />
        <div style="color: #999; font-size: 12px; margin-top: 4px;">
          {{ t('amnezia.expiryDaysPlaceholder') }}
        </div>
      </a-form-item>

      <a-form-item :label="t('amnezia.peerEnabled')">
        <a-switch v-model:checked="form.enabled" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>