<script setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  peerId: { type: Number, default: null },
  extend: { type: Function, required: true },
});

const emit = defineEmits(['update:open']);

const days = ref(30);
const loading = ref(false);

async function onSubmit() {
  if (days.value <= 0) return;
  loading.value = true;
  try {
    const ok = await props.extend(days.value);
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
    :title="t('amnezia.extendClient')"
    :confirm-loading="loading"
    @ok="onSubmit"
    @cancel="onClose"
  >
    <a-form layout="vertical">
      <a-form-item :label="t('amnezia.daysToAdd')">
        <a-input-number v-model:value="days" :min="1" :max="3650" style="width: 100%" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>