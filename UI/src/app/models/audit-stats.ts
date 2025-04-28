export interface AuditStats {
    clientStats: Array<{_id: string, count: number}>;
    modelStats: Array<{_id: string, count: number}>;
    clientTotal: number;
    modelTotal: number;
  }