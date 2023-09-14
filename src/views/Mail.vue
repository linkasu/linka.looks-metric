<template>
    <v-container grid-list-xs>
        <v-toolbar color="primary">
            <v-toolbar-items>
                
            <v-btn flat to="/users">
                <v-icon>mdi-arrow-left</v-icon>
            </v-btn>
        </v-toolbar-items>
            <v-toolbar-title>Отправка письма</v-toolbar-title>
        </v-toolbar>

        <v-form @submit.prevent>
            <v-select v-model="emails" :items="emails" disabled chips label="Адресаты" multiple></v-select>
            <v-text-field
                name="name"
                label="Тема письма"
                v-model="subject"
            ></v-text-field>
            <v-textarea
                label="Текст письма"
                textarea
                v-model="text"
            />
            <v-btn :color="colors[state]" type="submit" :disabled="state!==0" @click.once="send">Отправить</v-btn>
        </v-form>
    </v-container>
</template>

<script lang="ts" setup>
import { useUsersStore } from '@/store/users';
import { computed } from 'vue';
import { ref } from 'vue'

const state = ref(0)
const colors = ['accent', 'warming', 'success']
const userStore = useUsersStore()
const emails = computed(() => userStore.selected)

const subject = ref("")
const text = ref("")
async function send() {
    state.value=1
    await userStore.send(subject.value, text.value) 
    state.value=2
}

</script>