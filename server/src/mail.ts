import { readFile } from "fs/promises";
import { createTransport } from "nodemailer";
import { join } from "path";
import { Data, render } from "ejs";
    import * as url from 'url';
    // const __filename = url.fileURLToPath(import.meta.url);
    // const __dirname = url.fileURLToPath(new URL('.', import.meta.url));
export class Mail {
  private static get transport() {
    return createTransport({
      pool: true,
      host: "smtp.yandex.ru",
      port: 465,
      secure: true, // use TLS
      auth: {
        user: process.env.EMAIL,
        pass: process.env.EMAIL_PASSWORD,
      },
    });
  }

  static async sendActivationEmail(to: string, code: string) {
    return this.sendTemplate(to, 'Код активации для LINKa смотри. ' + code, "activation", { code })
  }
  private static async sendTemplate(to: string, subject: string, template: string, data: Data) {
    const templateEjs = await readFile(join(__dirname, '/../views/mails/' + template + '.ejs'), 'utf-8')
    const html = render(templateEjs, data)
    return this.sendEmail(to, subject, html)
  }
  static async sendEmail(to: string, subject: string, html: string) {
    var message = {
      from: '"LINKa" apps@linka.su',
      to,
      subject,
      html
    };
    const transport = Mail.transport
    const res = await transport.sendMail(message)
    transport.close()
    return res
  }
}
