import { Router } from "express";
import { Dashboard } from "../Dashboard";
import { Count, DataCount } from "./DataCount";
import { Mail } from "../mail";
import axios from "axios";

export const apiRouter = Router()

const dashboard = new Dashboard()
interface VKAuthTokenResponse {

    access_token: string,
    expires_in: number,
    user_id: number

}

const redirect_uri = "https://metric.linka.su/api/auth/callback";
const client_id = 4715265;
const ADMIN_ID = 20124065
const client_secret = "TWCm6iVoQj8crXbnYIQE";
apiRouter.get("/auth/url", (req, res) => {
    res.send({
        url: `https://oauth.vk.com/authorize?client_id=4715265&display=page&redirect_uri=${redirect_uri}&response_type=code&v=5.131`,
    });
});
apiRouter.get<{ code: string }>("/auth/callback", async (req, res) => {
    try {
        const response = await axios.get<VKAuthTokenResponse>("https://oauth.vk.com/access_token", {
            params: {
                client_id,
                client_secret,
                redirect_uri,
                code: req.query.code,
            },
        });
        if (response.data.user_id !== ADMIN_ID) {
            throw new Error('no admin')
        }
        (req.session as any).vktoken = response.data.access_token;
        (req.session as any).uid = response.data.user_id
        req.session.save(() => {
            res.redirect("/");

        })
    } catch (error) {
        res.send("Ошибка авторизации");
    }
});


apiRouter.get('/count', async (_, res) => {
    const users = await dashboard.getUsersCount()
    const pcs = await dashboard.getPcsCount()
    const events = await dashboard.getEventsCount()
    res.send({
        users,
        pcs,
        events
    })
})


apiRouter.get<{ from?: number, to?: number }>('/usersByPeriod', async (req, res) => {

    const now = new Date
    const from = req.query.from !== undefined ? new Date(+req.query.from) : new Date(+now - (60 * 1000 * 60 * 24 * 7))
    const to = req.query.to !== undefined ? new Date(+req.query.to) : now
    const count = await dashboard.getActiveUsers(from, to)
    res.send({ count } as Count)
})
apiRouter.get<{ from?: number, to?: number }>('/usersPerDay', async (req, res) => {
    const now = new Date
    const from = req.query.from !== undefined ? new Date(+req.query.from) : new Date(+now - (60 * 1000 * 60 * 24 * 7))
    const to = req.query.to !== undefined ? new Date(+req.query.to) : now
    const count = await dashboard.getUsersPerDay(from, to)
    res.send(Object.fromEntries(count))
})

apiRouter.get('/events', async (_, res) => res.send(await dashboard.getEventMatrix()))
apiRouter.get('/users', async (_, res) => res.send(await dashboard.getAllUsers()))

apiRouter.post<{}, { status: string }, { subject: string, body: string, emails: string[] }>('/send', async (req, res) => {
    const body = (req.body);
    for (const to of body.emails) {
        await Mail.sendEmail(to, body.subject, body.body)
    }
    res.send({ status: 'ok' })
})

apiRouter.get('/users/csv', async (_, res) =>{
    const users = await dashboard.getAllUsers()
    const fields = ['id', 'email']
    let r = fields.join(',')
    for (const user of users) {
        r+='\n'+fields.map((key)=>user[key]).join(',')
    }
    res.type('.csv').send(r)
})
