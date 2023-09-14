import { Pc, Prisma, User } from "@prisma/client";
import { prisma } from "./prisma.js";

export class Dashboard {
    getAllUsers(): Promise<(User|{pcs: Pc[]})[]> {
        return prisma.user.findMany({
            include:{
                pcs: true
            }
        })
    }
    getUsersCount() {
        return prisma.user.count()
    }
    getPcsCount() {
        return prisma.pc.count()
    }
    getEventsCount() {
        return prisma.event.count()
    }

    async getActiveUsers(from: Date, to: Date) {
        const rows = await prisma.event.findMany({
            where: {
                date: {
                    gte: from,
                    lte: to,
                },
            },
        });
        const ids: number[] = [];
        for (const row of rows) {
            if (!ids.includes(row.pcId)) {
                ids.push(row.pcId)
            }
        }
        return ids.length;
    }

    async getUsersPerDay(from: Date, to: Date) {
        const DAY = 1000*60*60*24
        const map = new Map<string, number>()

        let c = +from
        while(c<+to-DAY){
            const current = new Date(c);
             map.set(current.getDate()+'/'+current.getMonth(), await this.getActiveUsers(current, new Date(c+DAY)))
            c+=DAY
        }

        return map
    }
    async getEventMatrix(){
        const events = await prisma.event.findMany()
        const map:{[key in string]: number} = {}
        for (const event of events) {
            if(map[event.type]==undefined){
                map[event.type]=0
            }
            map[event.type]++
        }
        return map
    }

}