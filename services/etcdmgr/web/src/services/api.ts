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

export const getKeys = async (prefix?: string): Promise<EtcdKeysResponse> => {
  const response = await api.get<EtcdKeysResponse>('/list_keys', {
    params: { prefix },
  });
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