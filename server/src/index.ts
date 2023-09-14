import express, { Request, Response } from 'express';
import bodyParser from "body-parser";
import { Event } from "./Event.js";
import { User } from "./User.js"; // Assuming you have a User class with activation methods
import cors from 'cors'
import { join } from 'path';
import { apiRouter } from './routes/api.js';
import { sessionRouter } from './session.js';


const app = express();
app.use(cors({
    origin: '*'
}))
app.use(express.json());
const dir = join(__dirname, '/../../dist');
app.use(express.static(dir))
app.use(sessionRouter)

app.use('/api', apiRouter)

const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

app.post('/requestActivation', async (req: Request, res: Response) => {
    try {
        const { email } = req.body;

        if (!emailRegex.test(email)) {
            res.status(400).json({ error: 'Invalid email format.' });
            return;
        }

        await User.sendActivationEmail(email);
        res.status(200).json({ message: 'Activation email sent successfully.' });
    } catch (error) {
        console.error(error)
        res.status(400).json({ error: 'Bad request' });
    }
});


app.post('/activate', async (req: Request, res: Response) => {
    try {
        const { email, code } = req.body;

        const hash = await User.activate(email, code);
        res.status(200).json({ hash, message: 'Account activated successfully.' });
    } catch (error) {
        res.status(400).json({ error: 'Bad request' });
    }
});



app.post('/registerEvent', async (req: Request, res: Response) => {
    try {
        const { hash, eventName, eventData } = req.body;
        const version = req.headers['user-agent']?.match(/looks\/(\S*)/)?.[1]
        console.log(version);

        await Event.saveEvent(hash, eventName, eventData != null ? JSON.stringify(eventData) : undefined, version);
        res.status(200).json({ message: 'Event registered successfully.' });
    } catch (error) {
        res.status(400).json({ error: 'Bad request' });
    }
});

app.use((req, res, next) => {
   
    if (req.url.endsWith("/auth")) {
        return next();
    }

    if (req.url.includes(".")) {
        return next();
    }

    const vktoken = (req.session as any)?.vktoken;
    if (!vktoken) {
        res.redirect("/auth");
        return;
    }
    next();
});
app.get('/*', (req, res) => {

    res.sendFile("index.html", { root: dir });

})

app.listen(30812, () => {
    console.log('Server is running on port 30812');
});
