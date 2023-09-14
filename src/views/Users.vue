<template>
    <v-container grid-list-xs v-if="users">
        <v-toolbar color="primary">
            <v-toolbar-title>Пользователи</v-toolbar-title>
            <v-toolbar-items>
                <v-spacer/>
                <v-btn flat icon v-if="selected.length>0" to="/mail"><v-icon>mdi-send</v-icon></v-btn>
            </v-toolbar-items>
        </v-toolbar>
        <v-form>
            <v-radio-group v-model="onlyActive" direction="horizontal" inline>
                <v-radio label="Все пользователи" :value="null"></v-radio>
                <v-radio label="Только активные" :value="true"></v-radio>
                <v-radio label="Только неактивные" :value="false"></v-radio>
            </v-radio-group>
        </v-form>
        <v-data-table  :headers="headers" :items='users' show-select item-value="email" v-model="selected">
            <template v-slot:item.pcs="{ item }">
                {{ item.columns.pcs.length }}
            </template>

        </v-data-table>
    </v-container>
</template>

<script lang="ts" setup>
import { useUsersStore } from '@/store/users';
import { ref } from 'vue';
import { computed } from 'vue';
import { reactive } from 'vue';
import { VDataTable } from 'vuetify/labs/VDataTable'
import {  } from "vuetify";
import { DeepUser } from "server/src/routes/DataCount";

const usersStore = useUsersStore()
usersStore.loadUsers()

const selected = computed({
    get(){
        return usersStore.selected
    },
    set(v){
        usersStore.selected = v
    }
})

const onlyActive = ref<boolean | null>(null)

const headers = ref<any[]>([
    {
        title: 'ID',
        align: 'start',
        key: 'id',
    },
    { title: 'E-mail', align: 'end', key: 'email' },
    { title: 'pcs', align: 'end', key: 'pcs' },

] )

const users = computed(() => {
    if (onlyActive.value === null) return usersStore.users
    return usersStore.users?.filter((user) => !!user&& onlyActive.value === true ? user.pcs.length > 0 : user.pcs.length == 0)
})
</script>