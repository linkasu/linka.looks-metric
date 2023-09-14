<template>
    <v-container justify-center>
    <v-btn color="success" :href="url" block large>Войти через ВК</v-btn>
  
</v-container>
</template>

<script lang="ts" setup>
import axios from 'axios';
import  { ref } from 'vue'

const url = ref<string>()
axios
.get<{url: string}>('/api/auth/url')
.then(({data})=>{

    url.value = data.url
    
})

</script>