import { Pc, User } from "@prisma/client";

export interface DataCount {
    users: number;
    pcs: number
    events: number
}
export interface Count {
    count: number       
}

export type DeepUser = User& {pcs: Pc[]}