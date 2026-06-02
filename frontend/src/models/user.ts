export type UserRole = "admin" | "member";

export interface User {
  email: string;
  role: UserRole;
  addedAt: number;
  addedBy?: string;
}
