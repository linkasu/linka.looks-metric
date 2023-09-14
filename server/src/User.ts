import { randomUUID } from "crypto";
import { prisma } from "./prisma.js";
import { Mail } from "./mail.js";

export class User {
    private static regexPassword = /^(?=.*[A-Za-z])(?=.*\d)[A-Za-z\d]{8,}$/


    public static async sendActivationEmail(email: string) {
        let user = await prisma.user.findUnique({ where: { email } })
        if (!user) {
            user = await prisma.user.create({
                data: {
                    email
                }
            })
        }
        if (!user) throw new Error('no user')
        const code = this.generateCode()
        await prisma.activationMail.create({
            data: {
                userId: user.id,
                code
            }
        })
        await Mail.sendActivationEmail(email, code)
    }

    public static async activate(email: string, code: string) {
        const user = await prisma.user.findUniqueOrThrow({ where: { email } })
        const mail = await prisma.activationMail.findFirstOrThrow({
            where: {
                userId: user.id,
                code
            }
        })
        const hash = randomUUID()
        await prisma.pc.create({
            data: {
                userId: user.id,
                hash,
                version: ''
            }
        })
        await prisma.activationMail.delete({ where: { id: mail.id } })
        return hash
    }


    private static generateCode() {
        let res = ''
        for (let i = 0; i < 6; i++) {
            res += Math.round(Math.random() * 9)

        }
        return res
    }
}   