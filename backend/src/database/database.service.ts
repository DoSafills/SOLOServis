import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { PrismaClient } from '@prisma/client';
import { PrismaPg } from '@prisma/adapter-pg';

@Injectable()
export class DatabaseService
  extends PrismaClient
  implements OnModuleInit, OnModuleDestroy
{
  constructor(configService: ConfigService) {
    const connectionString =
      configService.get<string>('DATABASE_URL') ??
      `postgresql://${encodeURIComponent(configService.get<string>('DB_USER') ?? 'postgres')}:${encodeURIComponent(configService.get<string>('DB_PASSWORD') ?? '')}@${configService.get<string>('DB_HOST') ?? 'localhost'}:${configService.get<string>('DB_PORT') ?? '5432'}/${configService.get<string>('DB_NAME') ?? 'soloservis'}?schema=public`;

    super({ adapter: new PrismaPg({ connectionString }) });
  }

  async onModuleInit(): Promise<void> {
    await this.$connect();
    console.log('Database connection established');
  }

  async getProducts(limit = 5) {
    return this.product.findMany({ take: limit });
  }

  async onModuleDestroy(): Promise<void> {
    await this.$disconnect();
  }
}
