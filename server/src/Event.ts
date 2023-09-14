import { prisma } from './prisma.js';

export class Event {
    public static async saveEvent(hash: string, eventName: string, eventData?: string, version?: string): Promise<void> {
        const pc = await prisma.pc.findFirstOrThrow({ where: { hash } })
        if(version!=undefined && pc.version!=version){
            await prisma.pc.update({
                where:{hash},
                data: {version}
            })
        }
        await prisma.event.create({
            data: {
                pcId: pc.id,
                userId: pc.userId,
                type: eventName,
                content: eventData,
            }
        })
    }
}