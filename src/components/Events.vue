<template>
    <v-card title="События" v-if="metricStore.events">
        <v-card-text>
            <ul>
                <li v-for="(value, index) in metricStore.events">
                <b>{{ index }}:</b> {{ value }}
                </li>
            </ul>
        </v-card-text>
    </v-card>
</template>

<script lang="ts" setup>
import { useMetricStore } from '@/store/metric';
import  { ref } from 'vue'

const metricStore = useMetricStore()
</script>