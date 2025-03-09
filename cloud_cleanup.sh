#!/bin/bash
# teardown-all-resources.sh - Delete all resources in a GCP project

PROJECT_ID="financetracker-451021"  # Replace with your actual project ID

# Confirmation to proceed
echo "WARNING: This will delete ALL resources in project ${PROJECT_ID}."
echo "This action cannot be undone. Backup any important data before proceeding."
read -p "Type 'DELETE ALL RESOURCES' to confirm: " CONFIRM

if [ "$CONFIRM" != "DELETE ALL RESOURCES" ]; then
    echo "Teardown canceled."
    exit 1
fi

# Set the project
gcloud config set project ${PROJECT_ID}

# Delete GKE clusters
echo "Deleting all GKE clusters..."
CLUSTERS=$(gcloud container clusters list --format="value(name)")
for CLUSTER in $CLUSTERS; do
    gcloud container clusters delete ${CLUSTER} --zone=us-central1-a --quiet
done

# Delete Compute Engine instances
echo "Deleting all Compute Engine instances..."
gcloud compute instances list --format="value(name,zone)" | while read INSTANCE ZONE; do
    gcloud compute instances delete ${INSTANCE} --zone=${ZONE} --quiet
done

# Delete Compute Engine disks
echo "Deleting all Compute Engine disks..."
gcloud compute disks list --format="value(name,zone)" | while read DISK ZONE; do
    gcloud compute disks delete ${DISK} --zone=${ZONE} --quiet
done

# Delete Cloud Storage buckets
echo "Deleting all Cloud Storage buckets..."
gsutil ls -p ${PROJECT_ID} | while read BUCKET; do
    gsutil rm -r ${BUCKET}
done

# Delete Artifact Registry repositories
echo "Deleting all Artifact Registry repositories..."
gcloud artifacts repositories list --format="value(name)" | while read REPO; do
    gcloud artifacts repositories delete ${REPO} --location=us-central1 --quiet
done

# Delete Cloud Run services
echo "Deleting all Cloud Run services..."
gcloud run services list --platform managed --format="value(name)" | while read SERVICE; do
    gcloud run services delete ${SERVICE} --platform managed --region=us-central1 --quiet
done

# Delete Cloud Functions
echo "Deleting all Cloud Functions..."
gcloud functions list --format="value(name)" | while read FUNCTION; do
    gcloud functions delete ${FUNCTION} --region=us-central1 --quiet
done

# Delete Pub/Sub subscriptions and topics
echo "Deleting all Pub/Sub subscriptions..."
gcloud pubsub subscriptions list --format="value(name)" | while read SUB; do
    gcloud pubsub subscriptions delete ${SUB}
done

echo "Deleting all Pub/Sub topics..."
gcloud pubsub topics list --format="value(name)" | while read TOPIC; do
    gcloud pubsub topics delete ${TOPIC}
done

# Delete Cloud SQL instances
echo "Deleting all Cloud SQL instances..."
gcloud sql instances list --format="value(name)" | while read INSTANCE; do
    gcloud sql instances delete ${INSTANCE} --quiet
done

# Delete VPC networks (must delete all dependent resources first)
echo "Deleting all firewall rules..."
gcloud compute firewall-rules list --format="value(name)" | while read RULE; do
    gcloud compute firewall-rules delete ${RULE} --quiet
done

echo "Deleting all VPC networks..."
gcloud compute networks list --format="value(name)" | grep -v default | while read NETWORK; do
    gcloud compute networks delete ${NETWORK} --quiet
done

# Delete Cloud Build triggers
echo "Deleting all Cloud Build triggers..."
gcloud builds triggers list --format="value(id)" | while read TRIGGER; do
    gcloud builds triggers delete ${TRIGGER} --quiet
done

echo "Resource deletion complete."
echo "Note: Some resources might require manual cleanup if they have dependencies."