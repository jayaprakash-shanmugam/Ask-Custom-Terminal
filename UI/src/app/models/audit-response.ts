import { AuditEvent } from "./audit-event";

export interface AuditResponse {
    data: AuditEvent[];
    total: number;
    page: number;
    pageSize: number;
  }