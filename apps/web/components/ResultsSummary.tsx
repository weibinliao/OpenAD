import React from 'react';
import { Shield, AlertTriangle, Users, Clock, TrendingUp, TrendingDown } from 'lucide-react';
import { useI18n } from '../contexts/I18nContext';

interface Permission {
  path: string;
  trustee: string;
  rights: string;
  type: string;
  inherited: boolean;
}

interface ResultsSummaryProps {
  permissions: Permission[];
  scanTime: number;
}

export default function ResultsSummary({ permissions, scanTime }: ResultsSummaryProps) {
  const { t } = useI18n();

  const stats = {
    total: permissions.length,
    allow: permissions.filter(p => p.type === 'Allow').length,
    deny: permissions.filter(p => p.type === 'Deny').length,
    inherited: permissions.filter(p => p.inherited).length,
    explicit: permissions.filter(p => !p.inherited).length,
    highRisk: permissions.filter(p => p.rights.includes('Full Control')).length,
  };

  const riskLevel = stats.highRisk > stats.total * 0.3 ? 'high' :
                   stats.highRisk > stats.total * 0.1 ? 'medium' : 'low';

  const cards = [
    {
      title: t('totalPermissions'),
      value: stats.total,
      icon: Shield,
      color: 'blue',
      change: null
    },
    {
      title: t('highRisk'),
      value: stats.highRisk,
      icon: AlertTriangle,
      color: riskLevel === 'high' ? 'red' : riskLevel === 'medium' ? 'yellow' : 'green',
      change: null
    },
    {
      title: t('explicit'),
      value: stats.explicit,
      icon: Users,
      color: 'purple',
      change: { value: 12, trend: 'up' }
    },
    {
      title: t('scanTime'),
      value: `${scanTime}s`,
      icon: Clock,
      color: 'gray',
      change: null
    }
  ];

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
      {cards.map((card, index) => (
        <div key={index} className="bg-secondary/50 rounded-lg p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">{card.title}</p>
              <p className="text-2xl font-bold">{card.value}</p>
              {card.change && (
                <div className="flex items-center mt-1">
                  {card.change.trend === 'up' ? (
                    <TrendingUp className="w-4 h-4 text-green-500 mr-1" />
                  ) : (
                    <TrendingDown className="w-4 h-4 text-red-500 mr-1" />
                  )}
                  <span className={`text-xs ${
                    card.change.trend === 'up' ? 'text-green-500' : 'text-red-500'
                  }`}>
                    {card.change.value}%
                  </span>
                </div>
              )}
            </div>
            <div className={`p-2 rounded-lg ${
              card.color === 'blue' ? 'bg-blue-100 text-blue-600 dark:bg-blue-900 dark:text-blue-300' :
              card.color === 'red' ? 'bg-red-100 text-red-600 dark:bg-red-900 dark:text-red-300' :
              card.color === 'yellow' ? 'bg-yellow-100 text-yellow-600 dark:bg-yellow-900 dark:text-yellow-300' :
              card.color === 'green' ? 'bg-green-100 text-green-600 dark:bg-green-900 dark:text-green-300' :
              card.color === 'purple' ? 'bg-purple-100 text-purple-600 dark:bg-purple-900 dark:text-purple-300' :
              'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300'
            }`}>
              <card.icon className="w-5 h-5" />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
