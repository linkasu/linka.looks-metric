<template>
  <v-container grid-list-xs>
    <v-toolbar>
      <v-spacer></v-spacer>
      <v-text-field type="date" v-model="fromDate" solo dense label="Начало"/>
      <v-text-field type="date" v-model="toDate" label="Конец"/>
      <v-btn color="success" flat @click="metricStore.refresh()">Обновить</v-btn>
      <v-spacer></v-spacer>
      <v-toolbar-items v-if="metricStore.count">
        <v-btn flat to="/users">Пользователи: {{ metricStore.count.users }}</v-btn>
        <v-btn flat disabled>Компьютеры: {{ metricStore.count.pcs }}</v-btn>
        <v-btn flat disabled>События: {{ metricStore.count.events }}</v-btn>
      </v-toolbar-items>

    </v-toolbar>
    <v-row>
      <v-col md="6">
        <users-per-period></users-per-period>
      </v-col>
      <v-col md="6">
        <events></events>
      </v-col>
    </v-row>
  </v-container>
</template>

<script lang="ts" setup>
import Events from '@/components/Events.vue';
import UsersPerPeriod from '@/components/UsersPerPeriod.vue';
import { useMetricStore } from '@/store/metric';
import { ref } from 'vue';
import { computed } from 'vue';

const metricStore = useMetricStore()
useMetricStore().refresh()

const fromDate = computed({
  get() {
    return metricStore.from.toISOString().slice(0,10)
  },
  set(value: string) {
    metricStore.from = new Date(value)
  }
})
const toDate = computed({
  get() {
    return metricStore.to.toISOString().slice(0,10)
  },
  set(value: string) {
    metricStore.to = new Date(value)
  }
})

</script>
