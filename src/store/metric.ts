import { defineStore } from "pinia";
import { Count, DataCount } from "server/src/routes/DataCount";
import { ax } from "./ax";

interface MetricStoreState{
    count?: DataCount,
    users?: number,
    history?: {[key in string]: number},
    events?:{[key in string]: number}
    
    from: Date,
    to: Date
}

export const useMetricStore = defineStore('metrics', {
    state():MetricStoreState {
        return {
            to: new Date,
            from: new Date(+new Date - (1000*60*60*24*7+10))
        }
    },
    actions:{
        async refresh(){
            await this.refreshCount()
            await this.refreshUsersByPeriod()
            await this.refreshUsersPerDay()
            await this.refreshEvents()
        },
        async refreshCount(){
            this.count = ( (await ax.get('/count', {
                
            })).data )
            console.log(this.count);
            
        },
        async refreshUsersByPeriod(){
            this.users = (await ax.get<Count>('/usersByPeriod', {
                params:{
                    from: +this.from,
                    to: +this.to
                }
            })).data.count
            
        },
        async refreshUsersPerDay(){
            this.history = ((await ax.get('/usersPerDay', {
                params:{
                    from: +this.from,
                    to: +this.to
                }
            })).data)
            
        },
        async refreshEvents(){
            this.events =  ((await ax.get('/events', {
                params:{
                    from: +this.from,
                    to: +this.to
                }
            })).data)
            
        }
    }
})