import axios from 'axios';
import { GetStateResponse } from '../types/api';

const api = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
});

export const ping = async (): Promise<string> => {
  const response = await api.get<string>('/ping');
  return response.data;
};

export const getState = async (prefix?: string): Promise<GetStateResponse> => {
  const response = await api.get<GetStateResponse>('/get_state', {
    params: { prefix },
  });
  return response.data;
};

export const getShardPlan = async (): Promise<string> => {
  const response = await api.get<string>('/get_shard_plan', {
    responseType: 'text',
  });
  return response.data;
};

export const setShardPlan = async (shardPlan: string): Promise<string> => {
  const response = await api.post<string>('/set_shard_plan', shardPlan, {
    headers: {
      'Content-Type': 'text/plain',
    },
    responseType: 'text',
  });
  return response.data;
};

export interface ServiceConfig {
  shard_config?: {
    move_policy?: string;
    max_replica_count?: number;
    min_replica_count?: number;
  };
  worker_config?: {
    max_assignment_count_per_worker?: number;
    offline_grace_period_sec?: number;
  };
  system_limit?: {
    max_shards_count_limit?: number;
    max_replica_count_limit?: number;
    max_assignment_count_limit?: number;
    max_hat_count_count?: number;
    max_concurrent_move_count_limit?: number;
  };
  cost_func_cfg?: {
    shard_count_cost_enable?: number;
    shard_count_cost_norm?: number;
    worker_max_assignments?: number;
  };
  solver_config?: {
    soft_solver_config?: {
      solver_enabled?: number;
      run_per_minute?: number;
      explore_per_run?: number;
    };
    assign_solver_config?: {
      solver_enabled?: number;
      run_per_minute?: number;
      explore_per_run?: number;
    };
    unassign_solver_config?: {
      solver_enabled?: number;
      run_per_minute?: number;
      explore_per_run?: number;
    };
  };
  dynamic_threshold?: {
    dynamic_threshold_max?: number;
    dynamic_threshold_min?: number;
    half_decay_time_sec?: number;
    increase_per_move?: number;
  };
  fault_tolerance?: {
    grace_period_sec_before_drain?: number;
    grace_period_sec_before_dirty_purge?: number;
  };
}

export const getServiceConfig = async (): Promise<ServiceConfig> => {
  const response = await api.get('/get_service_config');
  return response.data;
};

export const setServiceConfig = async (config: Partial<ServiceConfig>): Promise<{ success: boolean; message: string }> => {
  const response = await api.post('/set_service_config', config);
  return response.data;
};