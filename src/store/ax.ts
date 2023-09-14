import axios, { Axios } from "axios";

export const ax =  axios.create({
    baseURL: '/api/',
    responseType: 'json'
});
