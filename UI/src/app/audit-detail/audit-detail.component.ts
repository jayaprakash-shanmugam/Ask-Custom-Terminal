import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { MatSnackBar } from '@angular/material/snack-bar';
import { AuditEvent } from '../models/audit-event';
import { AuditService } from '../services/audit.service';

@Component({
  selector: 'app-audit-detail',
  templateUrl: './audit-detail.component.html',
  styleUrls: ['./audit-detail.component.css']
})
export class AuditDetailComponent implements OnInit {
  auditType: 'client' | 'model' = 'client';
  auditId: string = '';
  audit: AuditEvent | null = null;
  loading = true;
  error: string | null = null;
  metadataKeys: string[] = [];

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private auditService: AuditService,
    private snackBar: MatSnackBar
  ) {}

  ngOnInit(): void {
    this.route.data.subscribe(data => {
      this.auditType = data['type'] || 'client';
    });

    this.route.params.subscribe(params => {
      this.auditId = params['id'];
      this.loadAuditDetails();
    });
  }

  loadAuditDetails(): void {
    this.loading = true;
    this.error = null;

    const getAudit = this.auditType === 'client' ? 
      this.auditService.getClientAuditById(this.auditId) : 
      this.auditService.getModelAuditById(this.auditId);

    getAudit.subscribe({
      next: (data) => {
        this.audit = data;
        if (this.audit.metadata) {
          this.metadataKeys = Object.keys(this.audit.metadata);
        }
        this.loading = false;
      },
      error: (err) => {
        if (err.status === 404) {
          this.error = `${this.auditType} audit not found.`;
        } else {
          this.error = `Failed to load ${this.auditType} audit details. Please try again.`;
        }
        this.loading = false;
        console.error(`Error loading ${this.auditType} audit:`, err);
      }
    });
  }

  goBack(): void {
    this.router.navigate([`/${this.auditType}s`]);
  }

  viewSession(sessionId: string): void {
    if (sessionId) {
      this.router.navigate(['/session', sessionId]);
    }
  }

  formatTimestamp(timestamp: Date | string | undefined): string {
    if (!timestamp) return 'N/A';
    return new Date(timestamp).toLocaleString();
  }

  copyToClipboard(text: string): void {
    navigator.clipboard.writeText(text).then(() => {
      this.snackBar.open('Copied to clipboard', 'Dismiss', {
        duration: 2000,
      });
    });
  }

  getMetadataValue(metadata: any, key: string): string {
    const value = metadata[key];
    if (value === null || value === undefined) return 'null';
    if (typeof value === 'object') return JSON.stringify(value, null, 2);
    return value.toString();
  }

  isJson(str: string): boolean {
    try {
      return typeof JSON.parse(str) === 'object';
    } catch (e) {
      return false;
    }
  }

  formatJson(str: string): string {
    try {
      return JSON.stringify(JSON.parse(str), null, 2);
    } catch (e) {
      return str;
    }
  }
}