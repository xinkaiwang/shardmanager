import axios from 'axios';
import { StatusResponse, EtcdKeyResponse, EtcdKeysResponse, EtcdKeyRequest } from '../types/api';

const api = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
});

export const getStatus = async (): Promise<StatusResponse> => {
  const response = await api.get<StatusResponse>('/status');
  return response.data;
};

export const getKeys = async (prefix?: string, limit?: number, nextToken?: string): Promise<EtcdKeysResponse> => {
  const params: Record<string, string | number> = {};
  
  if (prefix !== undefined && prefix !== null) {
    params.prefix = prefix;
  }
  
  if (limit !== undefined && limit > 0) {
    params.limit = limit;
  }
  
  if (nextToken !== undefined && nextToken !== null && nextToken !== '') {
    params.nextToken = nextToken;
  }
  
  console.log('API 请求参数:', {
    prefix: params.prefix,
    limit: params.limit,
    hasNextToken: params.nextToken ? true : false
  });
  
  const response = await api.get<EtcdKeysResponse>('/list_keys', { params });
  return response.data;
};

export const getKey = async (key: string): Promise<EtcdKeyResponse> => {
  const response = await api.get<EtcdKeyResponse>('/get_key', {
    params: { key },
  });
  return response.data;
};

export const setKey = async (key: string, value: string): Promise<void> => {
  await api.post(`/set_key?key=${encodeURIComponent(key)}`, { value } as EtcdKeyRequest);
};

export const deleteKey = async (key: string): Promise<void> => {
  await api.post(`/delete_key?key=${encodeURIComponent(key)}`);
}; 