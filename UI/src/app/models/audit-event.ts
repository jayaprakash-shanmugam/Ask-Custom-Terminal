
export interface AuditEvent {
    _id?: string;
    event_type: string;
    message: string;
    timestamp: Date;
    metadata: Record<string, any>;
  }