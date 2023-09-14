// import { User } from "@prisma/client";
import { defineStore } from "pinia";
import { ax } from "./ax";
import { User } from "@prisma/client";
import { DeepUser } from "server/src/routes/DataCount";
import axios from "axios";


interface UsersState {
    users?: DeepUser[],
    selected: string[]
}

export const useUsersStore = defineStore('users', {
    state():UsersState {
        return {
            selected: ['ibakaidov@ya.ru', 'ivan@aacidov.ru']
        }
    },
    actions:{
        async loadUsers(){
            this.users =[... (( await ax.get('/users')).data)]
        },
        async send(subject: string, body: string){
            await ax.post('/send', ( {
                subject,
                body,
                emails: this.selected
            }))
        }
    }
})