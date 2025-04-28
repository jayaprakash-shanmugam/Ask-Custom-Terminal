import { Component, OnInit } from '@angular/core';
import { ChartConfiguration } from 'chart.js';
import { AuditService } from '../services/audit.service';

@Component({
  selector: 'app-stats',
  templateUrl: './stats.component.html',
  styleUrls: ['./stats.component.css']
})
export class StatsComponent implements OnInit {
  loading = true;
  error: string | null = null;
  clientStats: any[] = [];
  modelStats: any[] = [];
  clientTotal = 0;
  modelTotal = 0;
  combinedStats: any[] = [];
  
  // Chart configurations
  eventTypeChart: ChartConfiguration = {
    type: 'pie',
    data: {
      labels: [],
      datasets: [{
        data: [],
        backgroundColor: [
          '#3f51b5', '#f44336', '#ff9800', '#4caf50', '#9c27b0',
          '#2196f3', '#ff5722', '#795548', '#607d8b', '#009688'
        ]
      }]
    },
    options: {
      responsive: true,
      plugins: {
        legend: {
          position: 'right',
        },
        title: {
          display: true,
          text: 'Event Types Distribution'
        }
      }
    }
  };

  sourceComparisonChart: ChartConfiguration = {
    type: 'bar',
    data: {
      labels: ['Client Events', 'Model Events'],
      datasets: [{
        data: [],
        backgroundColor: ['#3f51b5', '#f44336']
      }]
    },
    options: {
      responsive: true,
      plugins: {
        legend: {
          display: false
        },
        title: {
          display: true,
          text: 'Client vs Model Events'
        }
      },
      scales: {
        y: {
          beginAtZero: true
        }
      }
    }
  };

  // Timeline chart for events over time
  timelineChart: ChartConfiguration = {
    type: 'line',
    data: {
      labels: [],
      datasets: [
        {
          label: 'Client Events',
          data: [],
          borderColor: '#3f51b5',
          backgroundColor: 'rgba(63, 81, 181, 0.1)',
          tension: 0.2
        },
        {
          label: 'Model Events',
          data: [],
          borderColor: '#f44336',
          backgroundColor: 'rgba(244, 67, 54, 0.1)',
          tension: 0.2
        }
      ]
    },
    options: {
      responsive: true,
      plugins: {
        title: {
          display: true,
          text: 'Events Timeline'
        }
      },
      scales: {
        y: {
          beginAtZero: true,
          title: {
            display: true,
            text: 'Event Count'
          }
        },
        x: {
          title: {
            display: true,
            text: 'Time Period'
          }
        }
      }
    }
  };

  // Top models chart
  topModelsChart: ChartConfiguration = {
    type: 'bar',
    data: {
      labels: [],
      datasets: [{
        label: 'Events Count',
        data: [],
        backgroundColor: '#2196f3'
      }]
    },
    options: {
      responsive: true,
      indexAxis: 'y',
      plugins: {
        legend: {
          display: false
        },
        title: {
          display: true,
          text: 'Top Models by Usage'
        }
      },
      scales: {
        x: {
          beginAtZero: true
        }
      }
    }
  };

  constructor(private auditService: AuditService) {}

  ngOnInit(): void {
    this.loadStats();
  }

  loadStats(): void {
    this.loading = true;
    this.error = null;
    this.auditService.getAuditStats().subscribe({
      next: (stats) => {
        this.clientStats = stats.clientStats || [];
        this.modelStats = stats.modelStats || [];
        this.clientTotal = stats.clientTotal || 0;
        this.modelTotal = stats.modelTotal || 0;
        
        // Update chart data
        this.updateCharts(stats);
        
        this.loading = false;
      },
      error: (err) => {
        this.error = 'Failed to load audit statistics. Please try again.';
        this.loading = false;
        console.error('Error loading statistics:', err);
      }
    });
  }

  updateCharts(stats: any): void {
    // Update event type distribution chart
    this.combinedStats = [...this.clientStats, ...this.modelStats];
    const aggregatedStats: Map<string, number> = new Map();
    
    this.combinedStats.forEach(stat => {
      const eventType = stat._id;
      const count = stat.count || 0;
      aggregatedStats.set(eventType, (aggregatedStats.get(eventType) || 0) + count);
    });
    
    const labels = Array.from(aggregatedStats.keys());
    const data = Array.from(aggregatedStats.values());
    
    this.eventTypeChart.data.labels = labels;
    this.eventTypeChart.data.datasets[0].data = data;
    
    // Update source comparison chart
    this.sourceComparisonChart.data.datasets[0].data = [this.clientTotal, this.modelTotal];

    // Update timeline chart (assuming we have timeline data from the service)
    if (stats.timeline) {
      this.timelineChart.data.labels = stats.timeline.map((item: { period: string }) => item.period);
      this.timelineChart.data.datasets[0].data = stats.timeline.map((item: { clientCount: number }) => item.clientCount);
      this.timelineChart.data.datasets[1].data = stats.timeline.map((item: { modelCount: number }) => item.modelCount);
    }

    // Update top models chart
    if (stats.topModels) {
      this.topModelsChart.data.labels = stats.topModels.map((model: { name: string }) => model.name);
      this.topModelsChart.data.datasets[0].data = stats.topModels.map((model: { count: number }) => model.count);
    }
    
    // Deduplicate combinedStats based on _id
    const uniqueIds = new Set();
    this.combinedStats = this.combinedStats.filter(item => {
      if (!uniqueIds.has(item._id)) {
        uniqueIds.add(item._id);
        return true;
      }
      return false;
    });
  }

  refreshStats(): void {
    this.loadStats();
  }

  // Helper methods for table display
  getClientCount(eventType: string): number {
    const stat = this.clientStats.find(s => s._id === eventType);
    return stat ? stat.count || 0 : 0;
  }

  getModelCount(eventType: string): number {
    const stat = this.modelStats.find(s => s._id === eventType);
    return stat ? stat.count || 0 : 0;
  }

  getTotalCount(eventType: string): number {
    return this.getClientCount(eventType) + this.getModelCount(eventType);
  }

  getEventTypePercentage(eventType: string): number {
    const total = this.clientTotal + this.modelTotal;
    if (!total) return 0;
    
    const eventCount = this.getTotalCount(eventType);
    return Math.round((eventCount / total) * 100);
  }

  exportCSV(): void {
    const headers = ['Event Type', 'Client Count', 'Model Count', 'Total'];
    let csvContent = headers.join(',') + '\n';
    
    // Get all unique event types
    const eventTypes = new Set<string>();
    this.clientStats.forEach(stat => eventTypes.add(stat._id));
    this.modelStats.forEach(stat => eventTypes.add(stat._id));
    
    // Create rows for each event type
    eventTypes.forEach(eventType => {
      const clientCount = this.getClientCount(eventType);
      const modelCount = this.getModelCount(eventType);
      const totalCount = clientCount + modelCount;
      
      csvContent += `${eventType},${clientCount},${modelCount},${totalCount}\n`;
    });
    
    // Add totals row
    csvContent += `Total,${this.clientTotal},${this.modelTotal},${this.clientTotal + this.modelTotal}\n`;
    
    // Create and download the file
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    const url = URL.createObjectURL(blob);
    
    link.setAttribute('href', url);
    link.setAttribute('download', 'audit_stats.csv');
    link.style.visibility = 'hidden';
    
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }
}