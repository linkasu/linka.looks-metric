<template>
    <v-card title="Пользователи за период" v-if="chartData">
        <v-card-text>
            <Line id="my-chart-id" :options="chartOptions" :data="chartData" />
        </v-card-text>
    </v-card>
</template>

<script lang="ts" setup>
import { useMetricStore } from '@/store/metric';
import { Bar, Line } from 'vue-chartjs'
import { Chart as ChartJS, Title, Tooltip, Legend, BarElement, PointElement, LineElement, CategoryScale, LinearScale } from 'chart.js'
import { Point } from "chart.js";
import { ref } from 'vue';
import { computed } from 'vue';

ChartJS.register(Title, Tooltip, Legend, PointElement, LineElement, BarElement, CategoryScale, LinearScale)


const metricStore = useMetricStore()

const chartData = computed(() => {
    if (!metricStore.history) return null;

    return {
        labels: Object.keys(metricStore.history),

        datasets: [{
            label: 'Пользователи',
            backgroundColor: '#f87979', data: Object.values(metricStore.history)
        }]
    }
})
const chartOptions = ref({
    responsive: true
})
</script>