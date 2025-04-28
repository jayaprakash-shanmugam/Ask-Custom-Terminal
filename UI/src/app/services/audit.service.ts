// src/app/services/audit.service.ts
import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { AuditEvent } from '../models/audit-event';
import { AuditResponse } from '../models/audit-response';
import { environment } from '../../environments/environment';
@Injectable({
  providedIn: 'root'
})
export class AuditService {
  private apiUrl = environment.apiUrl;

  constructor(private http: HttpClient) { }

  // Get paginated client audits with optional filters
  getClientAudits(page: number = 1, pageSize: number = 10, filters?: any): Observable<AuditResponse> {
    let params = new HttpParams()
      .set('page', page.toString())
      .set('pageSize', pageSize.toString());
    
    if (filters) {
      if (filters.eventType) params = params.set('eventType', filters.eventType);
      if (filters.clientId) params = params.set('clientId', filters.clientId);
      if (filters.sessionId) params = params.set('sessionId', filters.sessionId);
      if (filters.startDate) params = params.set('startDate', filters.startDate);
      if (filters.endDate) params = params.set('endDate', filters.endDate);
    }

    return this.http.get<AuditResponse>(`${this.apiUrl}/audits/client`, { params });
  }

  // Get paginated model audits with optional filters
  getModelAudits(page: number = 1, pageSize: number = 10, filters?: any): Observable<AuditResponse> {
    let params = new HttpParams()
      .set('page', page.toString())
      .set('pageSize', pageSize.toString());
    
    if (filters) {
      if (filters.eventType) params = params.set('eventType', filters.eventType);
      if (filters.clientId) params = params.set('clientId', filters.clientId);
      if (filters.sessionId) params = params.set('sessionId', filters.sessionId);
      if (filters.startDate) params = params.set('startDate', filters.startDate);
      if (filters.endDate) params = params.set('endDate', filters.endDate);
    }

    return this.http.get<AuditResponse>(`${this.apiUrl}/audits/model`, { params });
  }

  // Get a single client audit by ID
  getClientAuditById(id: string): Observable<AuditEvent> {
    return this.http.get<AuditEvent>(`${this.apiUrl}/audits/client/${id}`);
  }

  // Get a single model audit by ID
  getModelAuditById(id: string): Observable<AuditEvent> {
    return this.http.get<AuditEvent>(`${this.apiUrl}/audits/model/${id}`);
  }

  // Get all audits for a specific session
  getSessionAudits(sessionId: string): Observable<{data: AuditEvent[], total: number}> {
    return this.http.get<{data: AuditEvent[], total: number}>(`${this.apiUrl}/audits/session/${sessionId}`);
  }

  // Get audit statistics
  getAuditStats(): Observable<any> {
    return this.http.get<any>(`${this.apiUrl}/audits/stats`);
  }
}