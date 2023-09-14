import { Router } from "express";
import { prisma } from "./prisma";
import { PrismaSessionStore } from "@quixo3/prisma-session-store";
import session from "express-session";


export const sessionRouter = Router()

sessionRouter.use(session({
    secret: process.env.SESSION_SECRET??"ytjgfdmx",
    store: new PrismaSessionStore(prisma, {
        checkPeriod: 2 * 60 * 1000, //ms
        dbRecordIdIsSessionId: true,
        dbRecordIdFunction: undefined,
    })
}))