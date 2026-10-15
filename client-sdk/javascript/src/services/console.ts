import type { Configuration } from '../../generated/runtime';
import {
  ConfigApi,
  UsersApi,
  UsersSessionsApi,
} from '../../generated/apis';
import type {
  Config,
  CreateConsoleUserInput,
  ResetConsoleUserPasswordInput,
  Session,
  UpdateConsoleUserInput,
  User,
} from '../../generated/models';
import { ATTACH_CONTROL_PLANE, type ElevatableService, type LayrClient } from '../core/client';
import { handleApiCall } from '../core/transport';

export interface ConsoleControlPlane {
  config: {
    get: () => Promise<Config>;
    update: (input: Config) => Promise<Config>;
  };
  users: {
    create: (input: CreateConsoleUserInput) => Promise<User>;
    delete: (userId: string) => Promise<void>;
    get: (userId: string) => Promise<User>;
    list: () => Promise<Array<User>>;
    reset_password: (userId: string, input: ResetConsoleUserPasswordInput) => Promise<void>;
    update: (userId: string, input: UpdateConsoleUserInput) => Promise<User>;
    sessions: {
      delete: (userId: string, sessionId: string) => Promise<void>;
      list: (userId: string) => Promise<Array<Session>>;
      revoke: (userId: string) => Promise<void>;
    };
  };
}

export interface ConsoleService extends ElevatableService<ConsoleControlPlane> {
  client: LayrClient;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: Configuration) => ConsoleControlPlane;
}

export function useConsole(client: LayrClient): ConsoleService {
  const service: ConsoleService = {
    client,

    [ATTACH_CONTROL_PLANE]: (elevatedConfig: Configuration): ConsoleControlPlane => ({
      config: {
        get: () => handleApiCall(() => new ConfigApi(elevatedConfig).consoleConfigGet()),
        update: (input: Config) =>
          handleApiCall(() => new ConfigApi(elevatedConfig).consoleConfigUpdate({ config: input })),
      },
      users: {
        create: (input: CreateConsoleUserInput) =>
          handleApiCall(() =>
            new UsersApi(elevatedConfig).consoleUsersCreate({ createConsoleUserInput: input })
          ),
        delete: (userId: string) =>
          handleApiCall(async () => {
            await new UsersApi(elevatedConfig).consoleUsersDelete({ userId });
          }),
        get: (userId: string) =>
          handleApiCall(() => new UsersApi(elevatedConfig).consoleUsersGet({ userId })),
        list: () => handleApiCall(() => new UsersApi(elevatedConfig).consoleUsersList()),
        reset_password: (userId: string, input: ResetConsoleUserPasswordInput) =>
          handleApiCall(async () => {
            await new UsersApi(elevatedConfig).consoleUsersResetPassword({
              userId,
              resetConsoleUserPasswordInput: input,
            });
          }),
        update: (userId: string, input: UpdateConsoleUserInput) =>
          handleApiCall(() =>
            new UsersApi(elevatedConfig).consoleUsersUpdate({
              userId,
              updateConsoleUserInput: input,
            })
          ),
        sessions: {
          delete: (userId: string, sessionId: string) =>
            handleApiCall(async () => {
              await new UsersSessionsApi(elevatedConfig).consoleUsersSessionsDelete({
                userId,
                sessionId,
              });
            }),
          list: (userId: string) =>
            handleApiCall(() =>
              new UsersSessionsApi(elevatedConfig).consoleUsersSessionsList({ userId })
            ),
          revoke: (userId: string) =>
            handleApiCall(async () => {
              await new UsersSessionsApi(elevatedConfig).consoleUsersSessionsRevoke({
                userId,
                body: {},
              });
            }),
        },
      },
    }),
  };

  return service;
}
