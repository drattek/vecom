package com.vegusa.ecommerce.config;

import org.springframework.amqp.core.*;
import org.springframework.amqp.rabbit.connection.ConnectionFactory;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.amqp.support.converter.Jackson2JsonMessageConverter;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.beans.factory.annotation.Qualifier;

@Configuration
public class RabbitConfig {
    // Exchange compartido entre orígenes (ERP y Nissan); el origen de cada mensaje
    // se distingue por routing key (prefijo "erp."/"nissan.") y por el campo "source" del payload.
    public static final String SYNC_EXCHANGE = "sync.exchange";
    public static final String STOCK_PAGE_PROCESSED_QUEUE = "stock.page.processed";
    public static final String STOCK_SYNC_COMPLETED_QUEUE = "stock.sync.completed";
    public static final String STOCK_SYNC_FAILED_QUEUE = "stock.sync.failed";
    public static final String ITEM_PAGE_PROCESSED_QUEUE = "item.page.processed";
    public static final String ITEM_SYNC_COMPLETED_QUEUE = "item.sync.completed";
    public static final String ITEM_SYNC_FAILED_QUEUE = "item.sync.failed";
    public static final String NISSAN_EXISTENCIAS_PAGE_PROCESSED_QUEUE = "nissan.existencias.page.processed";
    public static final String NISSAN_EXISTENCIAS_SYNC_COMPLETED_QUEUE = "nissan.existencias.sync.completed";
    public static final String NISSAN_EXISTENCIAS_SYNC_FAILED_QUEUE = "nissan.existencias.sync.failed";

    @Bean
    public TopicExchange syncExchange() {
        return new TopicExchange(
                SYNC_EXCHANGE,
                true,
                false
        );
    }

    @Bean
    public Jackson2JsonMessageConverter messageConverter(){
        return new Jackson2JsonMessageConverter();
    }

    @Bean
    public RabbitTemplate rabbitTemplate(ConnectionFactory connectionFactory, Jackson2JsonMessageConverter messageConverter){
        RabbitTemplate template = new RabbitTemplate(connectionFactory);

        template.setMessageConverter(messageConverter);
        return template;
    }

    @Bean
    public Queue stockPageProcessedQueue(){
        return QueueBuilder.durable(
                STOCK_PAGE_PROCESSED_QUEUE
        ).build();
    }

    @Bean
    public Queue stockSyncCompletedQueue(){
        return QueueBuilder.durable(
                STOCK_SYNC_COMPLETED_QUEUE
        ).build();
    }

    @Bean
    public Queue stockSyncFailedQueue(){
        return QueueBuilder.durable(
                STOCK_SYNC_FAILED_QUEUE
        ).build();
    }

    @Bean
    public Queue itemPageProcessedQueue(){
        return QueueBuilder.durable(
                ITEM_PAGE_PROCESSED_QUEUE
        ).build();
    }

    @Bean
    public Queue itemSyncCompletedQueue(){
        return QueueBuilder.durable(
                ITEM_SYNC_COMPLETED_QUEUE
        ).build();
    }

    @Bean
    public Queue itemSyncFailedQueue(){
        return QueueBuilder.durable(
                ITEM_SYNC_FAILED_QUEUE
        ).build();
    }

    @Bean
    public Binding stockPageProcessedBinding(
        @Qualifier("stockPageProcessedQueue") Queue pageProcessedQueue,
        TopicExchange syncExchange
    ){
    return BindingBuilder
            .bind(pageProcessedQueue)
            .to(syncExchange)
            .with("stock.page.processed");
    }

    @Bean
    public Binding stockSyncCompletedBinding(
        @Qualifier("stockSyncCompletedQueue") Queue syncCompletedQueue,
        TopicExchange syncExchange
    ){
    return BindingBuilder
            .bind(syncCompletedQueue)
            .to(syncExchange)
            .with("stock.sync.completed");
    }

    @Bean
    public Binding stockSyncFailedBinding(
        @Qualifier("stockSyncFailedQueue") Queue syncFailedQueue,
        TopicExchange syncExchange
    ){
    return BindingBuilder
            .bind(syncFailedQueue)
            .to(syncExchange)
            .with("stock.sync.failed");
    }

    @Bean
    public Binding itemPageProcessedBinding(
        @Qualifier("itemPageProcessedQueue") Queue itemPageProcessedQueue,
        TopicExchange syncExchange
    ){
    return BindingBuilder
            .bind(itemPageProcessedQueue)
            .to(syncExchange)
            .with("item.page.processed");
    }

    @Bean
    public Binding itemSyncCompletedBinding(
        @Qualifier("itemSyncCompletedQueue") Queue itemSyncCompletedQueue,
        TopicExchange syncExchange
    ){
    return BindingBuilder
            .bind(itemSyncCompletedQueue)
            .to(syncExchange)
            .with("item.sync.completed");
    }

    @Bean
    public Binding itemSyncFailedBinding(
        @Qualifier("itemSyncFailedQueue") Queue itemSyncFailedQueue,
        TopicExchange syncExchange
    ){
    return BindingBuilder
            .bind(itemSyncFailedQueue)
            .to(syncExchange)
            .with("item.sync.failed");
    }

    @Bean
    public Queue nissanExistenciasPageProcessedQueue(){
        return QueueBuilder.durable(
                NISSAN_EXISTENCIAS_PAGE_PROCESSED_QUEUE
        ).build();
    }

    @Bean
    public Queue nissanExistenciasSyncCompletedQueue(){
        return QueueBuilder.durable(
                NISSAN_EXISTENCIAS_SYNC_COMPLETED_QUEUE
        ).build();
    }

    @Bean
    public Queue nissanExistenciasSyncFailedQueue(){
        return QueueBuilder.durable(
                NISSAN_EXISTENCIAS_SYNC_FAILED_QUEUE
        ).build();
    }

    @Bean
    public Binding nissanExistenciasPageProcessedBinding(
            @Qualifier("nissanExistenciasPageProcessedQueue") Queue nissanExistenciasPageProcessedQueue,
            TopicExchange syncExchange
    ){
        return BindingBuilder
                .bind(nissanExistenciasPageProcessedQueue)
                .to(syncExchange)
                .with("nissan.existencias.page.processed");
    }

    @Bean
    public Binding nissanExistenciasSyncCompletedBinding(
            @Qualifier("nissanExistenciasSyncCompletedQueue") Queue nissanExistenciasSyncCompletedQueue,
            TopicExchange syncExchange
    ){
        return BindingBuilder
                .bind(nissanExistenciasSyncCompletedQueue)
                .to(syncExchange)
                .with("nissan.existencias.sync.completed");
    }

    @Bean
    public Binding nissanExistenciasSyncFailedBinding(
            @Qualifier("nissanExistenciasSyncFailedQueue") Queue nissanExistenciasSyncFailedQueue,
            TopicExchange syncExchange
    ){
        return BindingBuilder
                .bind(nissanExistenciasSyncFailedQueue)
                .to(syncExchange)
                .with("nissan.existencias.sync.failed");
    }
}
